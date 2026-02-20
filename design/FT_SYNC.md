# `ft sync`

Manually reconcile `.ft` files on disk with the database. The sync command is built incrementally — each phase adds behavior.

## Usage

```
ft sync
```

No arguments. Operates on the `fts/` directory and project root relative to where `ft init` was run.

## Output

Every file is printed with its status. A summary line always appears at the end.

### Files only

```
  new  fts/login.ft
  new  fts/checkout.ft
  trk  fts/signup.ft

synced 3 files
```

First sync with no `.ft` files present:

```
synced 0 files
```

### Files and scenarios

```
  new  fts/login.ft
       + @ft:1 User logs in
       + @ft:2 User fails login
  new  fts/checkout.ft
       + @ft:3 User completes purchase
  trk  fts/signup.ft

synced 3 files, 5 scenarios
```

File with syntax errors:

```
  err  fts/bad.ft — Scenario Outline is not supported (line 12)

synced 3 files, 5 scenarios
```

### Change detection

```
  new  fts/signup.ft
       + @ft:6 User signs up
  mod  fts/login.ft
       ~ @ft:1 User logs in
       + @ft:7 Password reset
  trk  fts/checkout.ft
  del  fts/old.ft
       - @ft:4 Legacy flow removed

synced 4 files, 8 scenarios
```

### Markers

| Marker | Meaning                                          | Color          |
|--------|--------------------------------------------------|----------------|
| `new`  | new file registered                               | green (2)      |
| `trk`  | already tracked, no changes                       | dim (2;2)      |
| `mod`  | existing file changed                             | yellow (3)     |
| `del`  | tracked file missing from disk                    | red (1)        |
| `err`  | file has syntax errors                            | bright red (9) |
| `+`    | new scenario                                      | green (2)      |
| `~`    | updated scenario (name or content changed)        | yellow (3)     |
| `-`    | removed scenario                                  | red (1)        |
| `@ft:` | scenario ID                                       | gray (8)       |

The color applies to the marker only. The `@ft:` ID is always gray. The summary line is uncolored.

## Behavior by Phase

### Phase 2: Register New Files

1. Scan `fts/` for `.ft` files
2. For each file not already tracked in the `files` table — insert a `files` record
   - `file_path` — relative path (e.g. `fts/login.ft`)
   - `created_at`, `updated_at` — current timestamp
3. Already-tracked files are skipped

Only file registration. No parsing, no scenario extraction.

### Phase 3: Parse Scenarios

After registering files, parse each tracked `.ft` file:

1. Parse the file to extract `Feature:` name, `Background:` block, and `Scenario:` blocks
2. For each scenario:
   - If it has an `@ft:<id>` tag matching a DB record — already tracked, skip
   - If it has no `@ft:` tag — insert a `scenarios` record, write `@ft:<id>` tag to the file
4. If the file contains syntax errors (`Scenario Outline:`, `Rule:`, `Examples:`) — write `# ft error:` comments to the top of the file and skip processing

The `@ft:<id>` tag is written as the first tag on the line immediately above `Scenario:`.

### Phase 7: Change Detection

Full reconciliation replaces the simple "skip already-tracked" logic:

1. Re-parse every tracked `.ft` file
2. Match scenarios between file and DB using `@ft:<id>` tags
3. Handle each case:
   - **Tagged, in DB** — update name, content, `updated_at`
   - **Tagged, unknown ID** — fall back to name matching within the file. If matched, re-associate and fix the tag. If not, treat as new.
   - **Untagged** — fall back to name matching. If matched, write the `@ft:<id>` tag. If not, insert new scenario.
   - **In DB, not in file** — scenario was removed:
     - Has active test links → rehydrate (write back to file)
     - Has status history, no test links → insert `removed` status
     - No history, no test links → delete the scenario row
4. Handle deleted files (tracked in DB, missing from disk):
   - Apply the same per-scenario removal logic
   - If any scenarios rehydrated → recreate the file
   - If none rehydrated → set `files.deleted = TRUE`

### Phase 8: Test Link Discovery

After file/scenario sync, scan the project directory for test links:

1. Walk the project directory tree
2. For files matching test patterns (`*_test.go`), scan for `@ft:<id>` tags
3. Skip `.git/` directory
4. Diff scan results against `test_links` table:
   - New link → insert
   - Existing link → update `updated_at`
   - Missing link → delete

## Daemon Interaction

If the daemon (`ftd`) is running when `ft sync` is invoked, the CLI pauses the daemon before syncing and resumes it after. This prevents conflicting writes to the database.

## Errors

- `fts/` directory does not exist — error, run `ft init` first
- `fts/ft.db` does not exist — error, run `ft init` first
- Syntax errors in `.ft` files — written as comments to the file, file skipped (Phase 3+)

## Idempotency

Running `ft sync` multiple times with no file changes produces no database changes and no file modifications.
