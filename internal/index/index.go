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

CREATE TABLE IF NOT EXISTS media_files (
	volume_id TEXT NOT NULL,
	path      TEXT NOT NULL,
	size      INTEGER NOT NULL,
	mtime     INTEGER NOT NULL,
	PRIMARY KEY (volume_id, path)
);

CREATE TABLE IF NOT EXISTS volumes (
	volume_id TEXT PRIMARY KEY,
	label     TEXT NOT NULL DEFAULT '',
	last_seen INTEGER NOT NULL DEFAULT 0
);
`

const insertSQL = `INSERT INTO files(path, size, mtime, blake3, hashed_at) VALUES(?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
	  size=excluded.size, mtime=excluded.mtime,
	  blake3=excluded.blake3, hashed_at=excluded.hashed_at`

const putMediaSQL = `INSERT INTO media_files(volume_id, path, size, mtime)
	VALUES(?, ?, ?, ?)
	ON CONFLICT(volume_id, path) DO UPDATE SET
	  size=excluded.size, mtime=excluded.mtime`

const putVolumeSQL = `INSERT INTO volumes(volume_id, label, last_seen) VALUES(?, ?, ?)
	ON CONFLICT(volume_id) DO UPDATE SET label=excluded.label, last_seen=excluded.last_seen`

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

// MediaRecord holds the size and mtime recorded for a file on a volume, enough
// to decide whether a re-import can skip re-hashing it.
type MediaRecord struct {
	Size  int64
	Mtime int64
}

// VolumeMedia returns every recorded file for volumeID keyed by relative path,
// so an import can test membership in memory rather than querying per file.
func (i *Index) VolumeMedia(volumeID string) (map[string]MediaRecord, error) {
	rows, err := i.db.Query(`SELECT path, size, mtime FROM media_files WHERE volume_id = ?`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]MediaRecord{}
	for rows.Next() {
		var path string
		var r MediaRecord
		if err := rows.Scan(&path, &r.Size, &r.Mtime); err != nil {
			return nil, err
		}
		out[path] = r
	}
	return out, rows.Err()
}

// PutMedia records that relpath on volume volumeID was processed at the given
// size and mtime, so a later import can skip re-hashing it.
func (i *Index) PutMedia(volumeID, relpath string, size, mtime int64) error {
	_, err := i.db.Exec(putMediaSQL, volumeID, relpath, size, mtime)
	return err
}

// PutVolume upserts the human-readable label and last-seen time for volumeID.
func (i *Index) PutVolume(volumeID, label string) error {
	_, err := i.db.Exec(putVolumeSQL, volumeID, label, time.Now().Unix())
	return err
}

// VolumeInfo holds display data for one cached volume.
type VolumeInfo struct {
	VolumeID  string
	Label     string
	FileCount int64
	LastSeen  time.Time
}

// Volumes returns every volume present in media_files LEFT JOINed with volumes,
// so caches created before naming still appear with a blank label.
func (i *Index) Volumes() ([]VolumeInfo, error) {
	rows, err := i.db.Query(`
		SELECT mf.volume_id, COALESCE(v.label, ''), COUNT(mf.path), COALESCE(MAX(v.last_seen), 0)
		FROM media_files mf
		LEFT JOIN volumes v ON mf.volume_id = v.volume_id
		GROUP BY mf.volume_id
		ORDER BY COALESCE(MAX(v.last_seen), 0) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VolumeInfo
	for rows.Next() {
		var vi VolumeInfo
		var lastUnix int64
		if err := rows.Scan(&vi.VolumeID, &vi.Label, &vi.FileCount, &lastUnix); err != nil {
			return nil, err
		}
		if lastUnix != 0 {
			vi.LastSeen = time.Unix(lastUnix, 0)
		}
		out = append(out, vi)
	}
	return out, rows.Err()
}

// ClearVolume removes all media_files rows for volumeID and its volumes row,
// returning the number of media_files rows deleted.
func (i *Index) ClearVolume(volumeID string) (int64, error) {
	tx, err := i.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM media_files WHERE volume_id = ?`, volumeID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM volumes WHERE volume_id = ?`, volumeID); err != nil {
		return 0, err
	}
	return n, tx.Commit()
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
