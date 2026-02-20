# `ft` — Feature Tracking System Design

## Components

1. **Feature Files** — Gherkin `.ft` files within the `fts/` directory, the source of truth for scenario descriptions. A file is a container for scenarios — the grouping may be thematic, arbitrary, or organizational.
2. **SQLite Database** — `fts/ft.db`, created by `ft init` in the current directory. The `fts/` directory and its `.ft` files are versioned in git but `ft.db` is gitignored. Stores metadata (status, timestamps, associations to files and scenarios)
3. **CLI (`ft`)** — User-facing command-line interface. The CLI implements all core logic (parsing, syncing, status tracking) and is developed first. All functionality is testable through the CLI before daemon integration.
4. **Daemon (`ftd`)** — Background file watcher that automates what the CLI does manually. Built after the CLI is stable, reusing the same core logic.

---

## Data Model

Tracking happens at two levels:

- **File** — corresponds to a single `.ft` file on disk. A file is a container for scenarios, not necessarily a single "feature."
- **Scenario** — corresponds to an individual `Scenario:` block within a file; each scenario is independently tracked with its own status. Every scenario is tagged in the file with `@ft:<id>` where `<id>` is the scenario's database primary key. The `@ft:` tag is placed as the first tag on the line immediately above the `Scenario:` line. `Background:` is supported. `Scenario Outline:`, `Rule:`, and `Examples:` are not supported and are treated as syntax errors. A scenario's content ends at the next keyword (`Scenario:`, `Background:`) or end of file, following standard Gherkin parsing rules. Doc strings and data tables within steps are supported.

### Database Schema (conceptual)

```
files
  id            INTEGER PRIMARY KEY
  file_path     TEXT UNIQUE
  content       TEXT            -- file-level content (Feature: line, description, Background: block), kept in sync on each parse
  deleted       BOOLEAN DEFAULT FALSE
  created_at    TIMESTAMP
  updated_at    TIMESTAMP

scenarios
  id            INTEGER PRIMARY KEY
  file_id       INTEGER REFERENCES files(id)
  name          TEXT            -- parsed from "Scenario:" line
  content       TEXT            -- full gherkin content of the scenario, kept in sync on each parse
  created_at    TIMESTAMP
  updated_at    TIMESTAMP

statuses
  id            INTEGER PRIMARY KEY
  scenario_id   INTEGER REFERENCES scenarios(id)
  status        TEXT
  changed_at    TIMESTAMP       -- when this status was set
```

The current status of a scenario is the most recent row in `statuses` for that scenario (by `changed_at`). This gives a full history of every status transition with timestamps.

When any CLI command accesses a scenario whose file has been deleted (all scenarios detached, `deleted = TRUE`), the file is recreated from stored scenario content before the command proceeds. This restores the file record, clears the `deleted` flag, and writes all detached scenarios back with their `@ft:<id>` tags.

The database uses WAL (Write-Ahead Logging) mode to allow concurrent reads from the CLI while the daemon writes.

---

## Interaction: CLI <-> User

```
ft init                                 Initialize ft in current directory (creates fts/ directory containing ft.db, adds fts/ft.db to .gitignore). All ft commands must be run from this directory.
ft list                                 List all tracked scenarios
ft list --status=<status>               Filter by scenario status
ft list --no-activity                   Show only scenarios with no status records
ft show <id>                            Display a scenario's gherkin content, metadata, and status history by its @ft:<id>
ft status                               Display a high-level project report (scenario counts by status)
ft status <id> <status>                 Update a scenario's status by its @ft:<id>
ft sync                                 Manually trigger a sync between files and DB. If the daemon is running, pauses it and waits for confirmation before syncing.
```

## Interaction: CLI <-> Database

- `ft list` queries files joined with scenarios to show scenario statuses
- `ft status` inserts a new status record for the specified scenario
- `ft sync` parses files, inserts/updates file and scenario records, writes `@ft:<id>` tags, and reconciles test links
- `ft show` reads scenario metadata from the DB, gherkin content from the `.ft` file

## Interaction: CLI <-> Feature Files

- `ft show` reads the `.ft` file directly from disk to display gherkin content
- `ft sync` scans `fts/` for `.ft` files, parses and reconciles them against the DB (see [FILE_CHANGES.md](FILE_CHANGES.md)). Also scans non-`.ft` files for `@ft:<id>` tags to discover and reconcile test links (see [TESTS.md](TESTS.md)).
- If a `.ft` file has a syntax error, the error is written to the top of the file as a comment (e.g. `# ft error: Scenario Outline is not supported (line 12)`). The file is not processed until the error is resolved and the comment is removed.

## Interaction: Daemon <-> Feature Files

- Watches the directory tree for `.ft` file changes (create, modify, rename, delete)
- See [FILE_CHANGES.md](FILE_CHANGES.md) for detailed behavior on each event type

## Interaction: Daemon <-> Database

- Writes all file-change events as DB updates at the scenario level
- On startup, performs a full reconciliation (diff filesystem state vs. DB state), re-parsing all `.ft` files

## Interaction: Daemon <-> CLI

- No direct communication — they coordinate through the shared SQLite DB
- CLI can check if the daemon is running (`ft daemon status`)
- CLI can start/stop the daemon (`ft daemon start`, `ft daemon stop`)
- The daemon writes a PID file so the CLI knows its state

---

## State Flow (eventual target)

The CLI implements all of this logic first. The daemon automates it later.

```
 .ft files (disk)
        |
        v
   +---------+    watches     +--------+
   |  Daemon  |<--------------|  Files  |
   |  (ftd)   |-------------->|        |
   +----+-----+   parses &    +--------+
        |        reconciles
        | writes (files + scenarios)
        v
   +---------+
   |  SQLite  |<---- ft CLI reads/writes
   |  ft.db   |
   +---------+
        ^
        |
   +----+-----+
   |  CLI (ft) |<---- User
   +----------+
```

---

See [STATUSES.md](STATUSES.md) for the status lifecycle.
See [TESTS.md](TESTS.md) for test associations.

This keeps the `.ft` files as the authoritative source for *what* scenarios exist, while the database owns *where things stand* per scenario. The daemon removes the burden of manual syncing by parsing `.ft` files and diffing scenarios against the DB. The CLI stays simple — it mostly just talks to the DB.
