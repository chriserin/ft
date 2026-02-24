# `ft` — Implementation Phases

CLI-first development. Each phase builds on the previous and is independently testable.

Tables and columns are added only when the phase requires them. Each phase lists its schema migrations.

`ft sync` is implemented incrementally — each phase adds the sync behavior it needs.

---

## Phase 1: Foundation

`ft init` and database setup.

- Create the `fts/` directory if it doesn't exist
- Initialize SQLite DB (`fts/ft.db`) with WAL mode if it doesn't exist
- Add migration system (see implementation/MIGRATIONS.md)
- Add `fts/ft.db` to `.gitignore` if not already present

**Schema**: `schema_version` table (managed by the migration system)

**Testable**: run `ft init`, verify directory and DB are created.

---

## Phase 2: `ft sync` — Register New Files

First implementation of `ft sync`. Scans for new `.ft` files and registers them.

- Scan `fts/` for `.ft` files
- For untracked files: insert `files` record
- Already-tracked files are skipped (no change detection yet)

**Schema migration**:
- Create `files` table (`id`, `file_path`, `created_at`, `updated_at`)

**Testable**: place `.ft` files in `fts/`, run `ft sync`, verify `files` records created.

---

## Phase 3: `ft sync` — Parse Scenarios

Extend `ft sync` to parse `.ft` files and extract scenarios.

- Parse `Feature:` line
- Parse `Scenario:` blocks — extract name and content
- Parse `Background:` blocks
- Parse existing `@ft:` tags
- Reject `Scenario Outline:`, `Rule:`, `Examples:` as syntax errors
- Scenario content ends at the next `Scenario:`, `Background:`, tag line preceding a `Scenario:`, or EOF — blank lines within scenarios are allowed
- Write syntax errors as comments to the top of the file
- For each scenario: insert `scenarios` record, write `@ft:<id>` tag to file

**Schema migration**:
- Create `scenarios` table (`id`, `file_id`, `name`, `created_at`, `updated_at`)

**Testable**: place `.ft` files in `fts/`, run `ft sync`, verify scenarios extracted, `@ft:` tags written, and syntax errors handled.

---

## Phase 4: `ft list`

Query and display tracked scenarios.

- List all tracked scenarios as a flat list (see design/FT_LIST.md)

**Schema**: none — queries existing tables.

**Testable**: sync files, verify `ft list` output.

---

## Phase 5: `ft show`

Display a single scenario.

- Look up scenario by `@ft:<id>`
- Display gherkin content (read from file) and metadata

**Schema**: none — reads from file on disk and existing tables.

**Testable**: `ft show 42`, verify output includes content.

---

## Phase 6: `ft status`

Scenario status management and project reporting (see design/FT_STATUS.md).

- `ft status` without arguments displays a high-level project report (scenario counts by status)
- `ft status <id> <status>` inserts a new `statuses` record for the scenario
- Accept any text as status
- `ft show` updated to display status history
- `ft list` updated to show current status and support `--status=<status>` and `--no-activity` filtering

**Schema migration**:
- Create `statuses` table (`id`, `scenario_id`, `status`, `changed_at`)

**Testable**: `ft status 42 accepted`, verify new status record. Run `ft status` with no arguments to verify report. Run `ft show` and `ft list` to confirm.

---

## Phase 7: `ft sync` — Change Detection

Extend `ft sync` with full reconciliation.

- Re-parse all tracked `.ft` files and detect changes
- Apply File Modified / File Deleted logic from FILE_CHANGES.md
- Match by `@ft:` tag, fall back to name, fall back to new
- Handle removed scenarios (insert `removed` status if history exists, delete row if not)
- Handle deleted files (set `deleted = TRUE`, skip already-deleted in reconciliation)

**Schema migration**:
- Add `content` column to `scenarios` — needed to rehydrate removed scenarios and recreate deleted files
- Add `deleted` column to `files` — needed to mark deleted files while preserving referential integrity

**Testable**: modify/add/delete `.ft` files manually, run `ft sync`, verify DB reflects changes.

---

## Phase 8: `ft.nvim` — Neovim Plugin

Integrate `ft` into Neovim (see design/NEOVIM.md).

- Virtual text status display next to `@ft:<id>` tags via extmarks
- Telescope picker to browse, filter, and jump to scenarios
- Buffer-local keymaps to set scenario status under cursor
- User commands: `:FtSync`, `:FtList`, `:FtStatus`
- Autocommands for virtual text refresh and optional sync-on-write
- `:checkhealth ft` for diagnostics

**Schema**: none — calls the `ft` CLI binary for all data access.

**Testable**: open a `.ft` file, verify virtual text appears. Use keymaps to set status, verify virtual text updates. Open Telescope picker, filter and jump to scenario.

---

## Phase 9: `ft sync` — Test Link Discovery

Extend `ft sync` with test link scanning.

- Scan non-`.ft` files for `@ft:<id>` tags (excluding `.gitignore` matches and binaries)
- Insert/update/delete `test_links` rows based on scan results
- Rehydrate scenarios with active test links when removed from `.ft` files
- `ft show` updated to display linked tests

**Schema migration**:
- Create `test_links` table (`id`, `scenario_id`, `file_path`, `line_number`, `updated_at`)

**Testable**: add `@ft:` comments to test files, run `ft sync`, verify `test_links` populated. Remove a scenario with test links, verify rehydration.

---

## Phase 10: File Recreation

Recreate deleted files when accessing detached scenarios.

- When any CLI command accesses a scenario whose file is deleted, recreate the file from stored `content`
- Restore all detached scenarios from that file, clear `deleted` flag
- Update `ft show` and `ft status` to trigger recreation

**Schema**: none — uses existing `content` column and `deleted` flag.

**Testable**: delete a `.ft` file, run `ft sync`, then `ft show <id>` — verify file is recreated.

---

## Phase 11: Daemon (`ftd`)

Automate sync via file watching.

- Watch `fts/` for `.ft` file changes, reuse sync logic per event
- Watch project directory (excluding `.gitignore`, binaries, `.ft` files) for test link changes
- PID file management
- `ft daemon start`, `ft daemon stop`, `ft daemon status`
- `ft sync` pauses daemon before running
- Startup reconciliation (full sync on daemon start)

**Schema**: none — reuses all existing tables.

**Testable**: start daemon, modify files, verify DB updates without manual sync.
