# bulk-publish Specification

## Purpose
TBD - created by archiving change reliable-bulk-publish. Update Purpose after archive.
## Requirements
### Requirement: Publish imports in batches while keeping Photos warm

`publish` SHALL import derivatives into Apple Photos in batches of a configurable size rather than one `osxphotos import` process per file, so a large run makes a handful of AppleScript sessions against Photos instead of thousands.
It SHALL leave Photos.app running for the whole run and pause a configurable duration between batches, rather than quitting Photos, because `osxphotos` drives Photos over AppleScript and a cold, still-loading library hangs.
The run SHALL remain resumable: derivatives imported in a committed batch are marked published, so a re-run continues from where it stopped.

#### Scenario: A large publish is split into batches

- **WHEN** `publish` has 1200 derivatives to import with a batch size of 500
- **THEN** it issues three import batches (500, 500, 200), pausing between them and never quitting Photos.app

#### Scenario: Interrupted run resumes

- **WHEN** a publish run imports two batches successfully and is then interrupted
- **THEN** the imported derivatives are marked published, and a re-run skips them and imports only the remainder

### Requirement: Published state is recorded only on a verified import

A derivative SHALL be marked published only when the osxphotos import report marks that file `imported` with no `error` and a non-empty uuid.
A file the report marks `error`, or that is absent from the report, SHALL be counted as failed and left unpublished, so a later run retries it.
A failed file within a batch SHALL NOT abort the batch or the run.

#### Scenario: Rejected file is not marked published

- **WHEN** an import batch report marks a file with `error` true (Photos rejected it)
- **THEN** that derivative is left unpublished and counted as failed, and the other files in the batch are still processed

#### Scenario: Successful file is marked published

- **WHEN** an import batch report marks a file `imported` true with a uuid
- **THEN** that derivative is marked published with that uuid

