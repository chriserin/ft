# `ft` — Statuses File

## Problem

`fts/ft.db` is gitignored (see [DESIGN.md](DESIGN.md)). On a fresh clone, the
database doesn't exist. `ft sync` recreates it transparently the moment it
runs (`OpenProjectStore` creates and migrates a fresh file if one isn't
there) — `ft init` plays no part in this beyond creating the new statuses
file described below. Most of the DB's content can be regenerated from the
`.ft` files themselves:

- **Files and scenarios** — reparsed from `fts/*.ft`. Scenario `id`s are
  already embedded in the files as `@ft:<id>` tags.
- **Test links** — rediscovered by scanning `_test.go` files for `@ft:<id>`
  comments (see [TESTS.md](TESTS.md)).

**Status history cannot be regenerated.** A status transition (`accepted`,
`in-progress`, `done`, `blocked`, `removed`, `modified`) is a fact about _when
a human or the system made a decision_ — nothing in a `.ft` file records that.
Once `fts/ft.db` is gone, that history is gone with it unless it's captured
somewhere durable and git-tracked.

This document proposes a small, git-tracked file that captures exactly that:
scenario `id` + status history, nothing else.

### A blocking prerequisite

Today, `ft sync` only respects an `@ft:<id>` tag if the _file_ is already
tracked in the DB (see `reconcileTrackedFile` in `cmd/sync.go`). When a file's
path isn't found in the `files` table — true on a fresh clone, for every file —
it's treated as brand new, and every scenario gets a **fresh,
sequentially-assigned id**, overwriting the tags already in the file. This
matches the documented behavior in [FILE_CHANGES.md](FILE_CHANGES.md) ("File
Created": a tag only counts as a match if it matches an _existing_ DB record).

For a statuses file to be useful, `id`s recovered from `.ft` files during a
rebuild must line up with the `id`s recorded in the statuses file. That means
the rebuild path needs a mode that **inserts scenarios with their tagged id**
rather than always assigning a fresh one. This is a real (small) behavior
change to `ft sync`, not just a new file format — called out explicitly under
[Other Operations Affected](#other-operations-affected).

---

## Format

CSV, one row per status event, append-only, **not** sorted or rewritten — new
events are appended to the end of the file. This keeps every write a
single-line diff instead of an insertion that reshuffles the file. A header
row is written once, when the file is created.

```
id,status,changed_at
1,accepted,2026-08-01T14:03:11Z
2,in-progress,2026-08-01T14:05:47Z
1,done,2026-08-03T09:12:00Z
3,removed,2026-08-04T16:40:22Z
```

Columns mirror the `statuses` table exactly (`scenario_id` → `id`, `status`,
`changed_at`), minus the table's own surrogate `id` column, which has no
meaning outside the DB. Written with `encoding/csv` (not hand-joined strings)
so any future status value containing a comma or quote is escaped correctly —
the fixed set of status values today never needs it, but the writer should not
assume that stays true forever.

**Path:** `fts/statuses.csv`, alongside `fts/ft.db`. Unlike `ft.db`, this file
is **not** added to `.gitignore` — it's meant to be committed.

---

## When it's created and written

- **Created** by `ft init`, as an empty file, at the same time `fts/ft.db` is
created. If it already exists (e.g. after a clone), `ft init` leaves it alone.
- **Appended to** every time `Store.InsertStatus` is called and succeeds — this
is the single choke point already used by:
  - `ft status <id> <status>` (manual transitions)
  - `ft sync`'s automatic `modified` / `removed` / `restored` transitions

Since `InsertStatus` is already the one place all status writes go through, the
file write can live right next to the DB write there rather than being threaded
through every call site individually.

One detail worth being deliberate about: `statuses.changed_at` is currently
DB-generated (`DEFAULT (datetime('now'))`). To guarantee the file and the DB
agree on the timestamp for a given event (not two independent `now()` calls
that could drift by a few ms — harmless for humans, but avoidable), the
timestamp should be generated once in Go and passed explicitly to both the SQL
insert and the file append.

Write mechanics: append via `os.OpenFile(path, O_APPEND|O_CREATE|O_WRONLY,
0o644)` + a trailing newline. Appends don't need the temp-file-then-rename
dance that whole-file rewrites (like `writeTagsToFile`) use, since nothing
already in the file is being modified.

---

## Other Operations Affected

- **`ft sync` — new-file scenario matching.** As described above, the "is this
file new to the DB" check needs a companion "does this scenario already carry
an `@ft:<id>` tag, and is that id unclaimed" check, independent of whether the
_file_ is tracked. There's an existing test (`@ft:36` in
`phase_3_parse_scenarios.ft`) that currently asserts the opposite — any tag on
an untracked file's scenario is stale and gets stripped, unconditionally. That
protection is still needed: a tag can be bogus (typo, copy-pasted from another
project, hand-edited) and must not silently collide with an existing scenario.
So the check has to be conditional: if the tag's id is present **and not
already claimed by a different `scenarios` row**, insert with that explicit id
(SQLite's `INTEGER PRIMARY KEY` accepts explicit values and will resume
autoincrement from `max(id)+1` afterward, so no separate sequence-reset step is
needed). If the id is already claimed by something else, fall back to today's
behavior — strip the tag, assign a fresh id. In the post-clone rebuild case
this is always safe, since the DB is empty and no id is ever claimed yet.
Untagged scenarios continue to get fresh ids as today.

- **Rebuild flow (new) — lives in `ft sync`, not `ft init`.** `ft init` never
touches `files`/`scenarios`/`test_links` today (see
[DESIGN.md](DESIGN.md#interaction-cli---database) — that's exclusively
`ft sync`'s job), and `OpenProjectStore` already creates and migrates a fresh
`ft.db` transparently the moment anything opens it, whether or not `ft init`
ran first. So there's no need to invent a separate "rebuild mode": deleting
`fts/ft.db` and running `ft sync` already recreates `files`/`scenarios` for
every tagged scenario, as an ordinary side effect of the new-file-tag fix
above — the DB is empty, so every tag is unclaimed and gets adopted. The one
genuinely new step is replaying `statuses.csv`, which normal `ft sync` runs
never do (they only *write* to it via `InsertStatus`). `ft sync` should check,
on every run, whether the `statuses` table is empty while `statuses.csv` has
rows — that combination is unambiguous: a brand-new project has both empty; a
rebuild has an empty table and a non-empty file. When it fires:
  1. (Already covered above) tagged scenarios get their original ids back as
  part of the normal file-parsing pass.
  2. Replay `statuses.csv` line by line, calling `InsertStatus` (or a
  bulk-insert equivalent) for each entry whose `id` now has a matching
  `scenarios` row, in file order, to rebuild the `statuses` table with
  original timestamps. Entries whose `id` has no matching row (see below) are
  skipped — not an error.
  3. Re-run the existing test-link scan (`syncTestLinks`) to rebuild
  `test_links` — this already works from source, no change needed.

  This same check needs to run in the opposite direction too: an **existing**
  project adopting this feature has the reverse mismatch — `statuses` already
  has rows (accumulated before `statuses.csv` existed), but the file is empty,
  since `ft init` just creates it fresh. Left unhandled, that history is
  exactly as unprotected against DB loss as if this feature had never shipped.
  So the same `ft sync` check goes both ways: DB empty + file has rows → replay
  into the DB (above); DB has rows + file is empty → export all of `statuses`,
  ordered by `changed_at`, into the file instead. Both directions are one-time
  events — once the two have met, ordinary `InsertStatus` calls keep them in
  lockstep, so neither check fires again on subsequent syncs.

- **Orphaned status history — resolved: skip, don't reconstruct.** A scenario
that was removed from its `.ft` file but had status history is kept in the DB
today (soft-delete: `removed` status, row retained) per
[FILE_CHANGES.md](FILE_CHANGES.md). Its `id` will still appear in
`statuses.csv` after that removal (the existing `removed` entry already
reserves the id against reuse on rebuild — no separate mechanism needed for
that), but **its name and content are nowhere on disk** once the `Scenario:`
block is gone from the file — that information genuinely doesn't exist outside
`ft.db`.

  Rather than reconstructing a stand-in for it, the rebuild simply doesn't
  recreate that history: replay skips any `statuses.csv` entry whose `id` has
  no corresponding `scenarios` row once the file-parsing pass is done, leaving
  no `statuses` row and no `scenarios` row for it at all. The line stays in
  `statuses.csv` untouched — since the file is append-only and never
  rewritten — so nothing is lost from the file itself, only from what gets
  replayed into the fresh DB.

  This history is only actually at risk if `fts/ft.db` is deleted (or lost) —
  the ordinary case, where the DB is intact, is completely unaffected, since
  none of this rebuild logic runs. Losing a slice of already-removed-scenario
  history in that rare case is an acceptable tradeoff against the complexity
  of reconstructing placeholder rows (and updating `ft show`/`ft list` to
  render scenarios with no name or content) for something that, by
  definition, no longer exists in any tracked file.

  (A scenario with **no** status history at all is hard-deleted via
  `store.DeleteScenario` without ever touching `Store.InsertStatus`, so it
  leaves no trace in `statuses.csv` in the first place — same outcome, for the
  same underlying reason: nothing else in the system references it.)

- **`ft init`.** Needs only the new "create `statuses.csv` if absent" step —
no routing logic, no awareness of rebuild vs. fresh project. That distinction
is `ft sync`'s to make, via the empty-table-but-non-empty-file check above.

- **`.gitignore` handling (`ensureGitignore`).** No change needed for `ft.db`'s
entry, but nothing should ever add `statuses.csv` to it.

- **Daemon (`ftd`), when built.** Any status-changing code path in the daemon
must go through the same `InsertStatus` choke point (or an equivalent shared
helper) so it can't write to the DB without also appending to the file.

- **Concurrent writers.** `O_APPEND` writes are atomic for small writes on
POSIX filesystems, so CLI and (eventually) daemon appends won't corrupt each
other. No locking scheme is needed beyond what SQLite's WAL mode already
provides for the DB side.
