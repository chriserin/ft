# `ft` — Migration System

Go-based migration system for managing SQLite schema changes across phases.

---

## Overview

Migrations are embedded in the Go binary and run automatically on `ft init` and whenever the CLI opens the database. Each migration runs once, in order.

## Version Tracking

A single-row `schema_version` table tracks the current version:

```sql
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);
INSERT INTO schema_version VALUES (0);
```

On startup, compare the stored version against the number of available migrations and run any that are pending.

## Migration Format

Each migration is a Go file with an embedded SQL string, registered in order:

```go
package migrations

var All = []string{
    createFilesSQL,      // version 1
    createScenariosSQL,  // version 2
    createStatusesSQL,   // version 3
    // appended as needed
}
```

SQL is embedded as constants or via `//go:embed` from `.sql` files. Each migration contains the full SQL to apply (no down/rollback migrations).

## Execution

```go
func Migrate(db *sql.DB) error {
    // 1. Create schema_version table if not exists, insert 0 if empty
    // 2. Read current version from schema_version
    // 3. For each migration with index >= current version:
    //    a. Begin transaction
    //    b. Execute SQL
    //    c. UPDATE schema_version SET version = index + 1
    //    d. Commit
    // 4. Return error on any failure (transaction rolls back)
}
```

- Each migration runs in its own transaction
- If a migration fails, it rolls back and stops — no partial state

## When Migrations Run

- `ft init` — creates the DB and runs all migrations
- Any `ft` command that opens the DB — checks for and applies pending migrations

This ensures the schema is always up to date regardless of how the user updates the binary.

New migrations are appended as needed. Versions are sequential and never reordered.
