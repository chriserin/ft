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

## Phase 9: `ft sync` — Modified Status

Extend `ft sync` to automatically set a scenario's status to "modified" when its content changes (see design/MOD.md).

- When content has changed during reconciliation, insert a `modified` status record
- Name changes alone do not trigger `modified`
- Skip if the scenario's latest status is already `modified`
- Skip on restored scenarios (they already get a `restored` status)

**Schema**: none — uses existing `statuses` table.

**Testable**: change scenario steps, run `ft sync`, verify `modified` status inserted. Rename scenario without changing steps, verify no `modified` status.

---

## Phase 10: `ft sync` — Test Link Discovery

Extend `ft sync` with test link scanning.

- Scan non-`.ft` files for `@ft:<id>` tags (excluding `.gitignore` matches and binaries)
- Insert/update/delete `test_links` rows based on scan results
- Rehydrate scenarios with active test links when removed from `.ft` files
- `ft show` updated to display linked tests

**Schema migration**:
- Create `test_links` table (`id`, `scenario_id`, `file_path`, `line_number`, `updated_at`)

**Testable**: add `@ft:` comments to test files, run `ft sync`, verify `test_links` populated. Remove a scenario with test links, verify rehydration.

---

## Phase 11: File Recreation

Recreate deleted files when accessing detached scenarios.

- When any CLI command accesses a scenario whose file is deleted, recreate the file from stored `content`
- Restore all detached scenarios from that file, clear `deleted` flag
- Update `ft show` and `ft status` to trigger recreation

**Schema**: none — uses existing `content` column and `deleted` flag.

**Testable**: delete a `.ft` file, run `ft sync`, then `ft show <id>` — verify file is recreated.

---

## Phase 12: Daemon (`ftd`)

Automate sync via file watching.

- Watch `fts/` for `.ft` file changes, reuse sync logic per event
- Watch project directory (excluding `.gitignore`, binaries, `.ft` files) for test link changes
- PID file management
- `ft daemon start`, `ft daemon stop`, `ft daemon status`
- `ft sync` pauses daemon before running
- Startup reconciliation (full sync on daemon start)

**Schema**: none — reuses all existing tables.

**Testable**: start daemon, modify files, verify DB updates without manual sync.

---

## Phase 13: Statuses File

Make each scenario's current status survive a fresh clone, even though `fts/ft.db` is gitignored (see design/STATUSES_FILE.md). This is a snapshot of current status, not a full history log — full transition history remains DB-only.

- Create `fts/statuses.csv` in `ft init` if it doesn't already exist — git-tracked, unlike `ft.db`, so it must not be added to `.gitignore`. Header is `id,status`, no timestamp
- `Store.InsertStatus` upserts a CSV row (update the scenario's existing row if it has one, insert a new one in sorted-by-id position otherwise) whenever a status is recorded — manual (`ft status`), and `ft sync`'s automatic `modified` / `removed` / `restored` transitions
- Fix `ft sync`'s new-file path: a scenario carrying an existing `@ft:<id>` tag is inserted with that explicit id instead of always getting a fresh one, *unless that id is already claimed by a different scenario in the DB* — in which case today's behavior (strip the tag, assign fresh) still applies, preserving the existing stale-tag protection (`@ft:36` in phase_3_parse_scenarios.ft; the unclaimed-tag counterpart is `@ft:208` in phase_13_statuses_file.ft) while enabling the post-clone case, where the DB is empty and every tag is by definition unclaimed
- `ft sync` is what performs the rebuild, not `ft init` — `ft init` only creates `fts/statuses.csv`. `OpenProjectStore` already creates and migrates a fresh `ft.db` transparently if the file is missing, and the new-file-tag fix above already recreates every tagged scenario as an ordinary side effect of `ft sync`'s existing reconciliation — no separate "rebuild mode" is needed for that part
- The two sync directions are asymmetric, not both gated the same way: **DB → file runs unconditionally on every sync**, fully rewriting `fts/statuses.csv` from each scenario's current status in the DB, so the file always reflects the DB accurately — this is what backfills status history that predates `statuses.csv` (an existing project adopting this feature), and it self-heals any other drift too. Gating this on "only if the file is empty" was tried first and was wrong: in practice the file almost always already has *some* rows from ordinary `ft status` use, so that condition essentially never re-fired, and backfilling silently never happened for most of a project's pre-existing history
- **File → DB stays gated on "the `statuses` table has zero rows"** — that's the real, unambiguous signal that `fts/ft.db` was lost and this is a rebuild. Running it whenever the file simply has rows the DB doesn't (not a meaningful signal on its own) would risk inserting duplicate or conflicting history into an otherwise-healthy DB. Replay restores each scenario's current status only, using the rebuild's own timestamp — there's no original timestamp to recover
- For a status row whose id has no corresponding scenario anywhere on disk (removed from its file before the clone), the replay step skips that entry rather than reconstructing a placeholder — the row stays inert in `fts/statuses.csv` but is not recreated in the fresh DB. Acceptable since it only matters if `fts/ft.db` is lost, which is rare. The test-link scan then re-runs as usual to rebuild `test_links`

**Schema**: none — `fts/statuses.csv` is a git-tracked file alongside `ft.db`, not a new table.

**Testable**: run `ft status`, verify `fts/statuses.csv` gains a row. Delete `fts/ft.db` and rerun `ft sync` (not `ft init`), verify the DB and status history are rebuilt with the same ids and statuses as before.

---

## Phase 14: `ft agent-instructions`

Print AI-facing instructions for using `ft` — command reference and status vocabulary, written as behavioral rules rather than documentation (see design/AGENT_INSTRUCTIONS.md).

- `ft agent-instructions` takes no arguments, always prints to stdout, and does not require `ft init` to have been run first — it must not fail in an uninitialized project. It never touches the DB or `fts/` at all
- `ft init` does not mention this command in its output
- The output is a static Go string constant covering: core concepts (`@ft:<id>` tags, statuses, the gitignored DB vs. the git-tracked statuses file), a command reference phrased as instructions ("run `ft sync` after editing..."), and the status vocabulary split into three tiers by who's allowed to set each one:
  - System-set, nobody sets these via `ft status`: `modified`, `removed`, `restored`
  - Human-set, the agent must never set these: `ready`, `accepted`, `rejected`
  - Agent-set: `in-progress`, `fulfilled` — this is as far as the agent's own authority goes; it never sets `accepted` itself, even when confident its own testing is correct
- A workflow section ties the vocabulary together: never implement a scenario unless it's currently `ready`; mark `in-progress`, implement with a linked test, verify the test passes, mark `fulfilled` and stop. On `rejected`, the agent's involvement ends until a human moves it back to `ready` — directly, or by editing the spec (which `ft sync` turns into `modified` first, still requiring a human to promote it to `ready`). The agent never fixes and re-fulfills a rejected scenario on its own initiative
- No per-project override mechanism — considered and cut, see design/AGENT_INSTRUCTIONS.md. The intended usage (`ft agent-instructions >> CLAUDE.md`) already gives a human an editable copy without needing `ft` itself to support overrides

**Schema**: none — reads no DB state; the output is a static string.

**Testable**: run `ft agent-instructions` with no `fts/` directory present, verify it succeeds and prints the built-in text. Run `ft init`, verify its output never mentions `agent-instructions`.
