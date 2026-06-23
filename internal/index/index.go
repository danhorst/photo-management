// Package index stores content hashes of library files in a SQLite database,
// keyed for fast duplicate lookup and incremental rescans.
package index

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS files (
	path      TEXT PRIMARY KEY,
	size      INTEGER NOT NULL,
	mtime     INTEGER NOT NULL,
	blake3    TEXT NOT NULL,
	hashed_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_files_blake3 ON files(blake3);
`

const insertSQL = `INSERT INTO files(path, size, mtime, blake3, hashed_at) VALUES(?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
	  size=excluded.size, mtime=excluded.mtime,
	  blake3=excluded.blake3, hashed_at=excluded.hashed_at`

// batchSize bounds how many rows accumulate per transaction during a bulk
// index build, so an interrupted run keeps already-committed batches.
const batchSize = 2000

// Index is a handle to the on-disk content-hash database.
type Index struct {
	db *sql.DB
}

// Open opens (creating if needed) the index at path and applies the schema.
func Open(path string) (*Index, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Single writer/reader process: one connection keeps the per-connection
	// pragmas below in effect and avoids lock contention.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Index{db: db}, nil
}

// Close closes the underlying database.
func (i *Index) Close() error { return i.db.Close() }

// Lookup returns a stored path that has the given content hash, if any.
func (i *Index) Lookup(hash string) (path string, found bool, err error) {
	err = i.db.QueryRow(`SELECT path FROM files WHERE blake3 = ? LIMIT 1`, hash).Scan(&path)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

// Cached returns the stored hash for path when its size and mtime are unchanged,
// letting a rescan skip re-hashing unmodified files.
func (i *Index) Cached(path string, size, mtime int64) (hash string, ok bool) {
	var s, m int64
	err := i.db.QueryRow(`SELECT size, mtime, blake3 FROM files WHERE path = ?`, path).Scan(&s, &m, &hash)
	if err != nil || s != size || m != mtime {
		return "", false
	}
	return hash, true
}

// Put inserts or updates the record for path in its own transaction. Suitable
// for the handful of writes during an import; use Begin for bulk indexing.
func (i *Index) Put(path string, size, mtime int64, hash string) error {
	_, err := i.db.Exec(insertSQL, path, size, mtime, hash, time.Now().Unix())
	return err
}

// Stats returns the number of indexed files and the most recent hash time.
func (i *Index) Stats() (count int64, last time.Time, err error) {
	var lastUnix sql.NullInt64
	err = i.db.QueryRow(`SELECT COUNT(*), MAX(hashed_at) FROM files`).Scan(&count, &lastUnix)
	if lastUnix.Valid {
		last = time.Unix(lastUnix.Int64, 0)
	}
	return count, last, err
}

// Batch buffers Put calls in transactions of batchSize rows, committing each so
// a bulk index build avoids one fsync per row while staying resumable.
type Batch struct {
	idx  *Index
	tx   *sql.Tx
	stmt *sql.Stmt
	n    int
}

// Begin starts a batched write. Call Commit when done, or Rollback to discard
// the current (uncommitted) batch; already-committed batches persist.
func (i *Index) Begin() (*Batch, error) {
	b := &Batch{idx: i}
	if err := b.start(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Batch) start() error {
	tx, err := b.idx.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return err
	}
	b.tx, b.stmt, b.n = tx, stmt, 0
	return nil
}

// Put adds a row, committing and reopening the transaction every batchSize rows.
func (b *Batch) Put(path string, size, mtime int64, hash string) error {
	if _, err := b.stmt.Exec(path, size, mtime, hash, time.Now().Unix()); err != nil {
		return err
	}
	b.n++
	if b.n >= batchSize {
		if err := b.tx.Commit(); err != nil {
			return err
		}
		return b.start()
	}
	return nil
}

// Commit flushes the final partial batch.
func (b *Batch) Commit() error { return b.tx.Commit() }

// Rollback discards the current uncommitted batch.
func (b *Batch) Rollback() error { return b.tx.Rollback() }
