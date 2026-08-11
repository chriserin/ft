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

**Status cannot be regenerated.** A status (`accepted`, `in-progress`,
`done`, `blocked`, `removed`, `modified`) is a fact about a decision a human
or the system made — nothing in a `.ft` file records that. Once `fts/ft.db`
is gone, that's gone with it unless it's captured somewhere durable and
git-tracked.

This document proposes a small, git-tracked file that captures exactly that:
each scenario's `id` and its **current** status — a snapshot, not a full
history. Full status history (every transition, with timestamps) continues to
live only in `ft.db`; this file exists purely so a rebuild has *something* to
restore, not so it can reconstruct the exact timeline. That's a deliberate
scope cut: recording every event would let a rebuild restore full history,
but at the cost of an ever-growing file and upsert complexity that isn't
worth it for what's meant to be a last-resort safety net, not a second
database.

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

CSV, **one row per scenario that has ever had a status set**, holding its
current status only — not a log of every transition. A header row is written
once, when the file is created.

```
id,status
1,accepted
2,in-progress
3,removed
```

Only `id` and `status` — no `changed_at`. Since this is a snapshot rather than
an event log, there's no "when" to record: writing a status means updating
that `id`'s existing row, or inserting a new one if it doesn't have one yet.
Rows are kept sorted by `id`, so the file reads as a stable, predictable
table rather than an arbitrary event order — a single status change still
touches exactly one line either way (an update in place, or a single-line
insertion at the sorted position), so this stays just as git-diff-friendly as
a pure append would have been.

Written with `encoding/csv` (not hand-joined strings) so any future status
value containing a comma or quote is escaped correctly — the fixed set of
status values today never needs it, but the writer should not assume that
stays true forever.

**Path:** `fts/statuses.csv`, alongside `fts/ft.db`. Unlike `ft.db`, this file
is **not** added to `.gitignore` — it's meant to be committed.

---

## When it's created and written

- **Created** by `ft init`, as an empty file (header only), at the same time
`fts/ft.db` is created. If it already exists (e.g. after a clone), `ft init`
leaves it alone.
- **Upserted** every time `Store.InsertStatus` is called and succeeds — this
is the single choke point already used by:
  - `ft status <id> <status>` (manual transitions)
  - `ft sync`'s automatic `modified` / `removed` / `restored` transitions

Since `InsertStatus` is already the one place all status writes go through,
the file write can live right next to the DB write there rather than being
threaded through every call site individually.

Write mechanics: unlike a pure append, an upsert may need to replace an
existing line rather than only add one, so it needs the same
read-whole-file → modify → write-to-temp → rename dance that `writeTagsToFile`
already uses for `.ft` files, rather than a cheap `O_APPEND` open. See
[Concurrent writers](#other-operations-affected) below for what this costs.

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
genuinely new step is replaying `statuses.csv` into the (empty) `statuses`
table:
  1. (Already covered above) tagged scenarios get their original ids back as
  part of the normal file-parsing pass.
  2. Replay `statuses.csv`: for each `(id, status)` row whose `id` now has a
  matching `scenarios` row, insert one status record for it — there's no
  original timestamp to restore, so this uses the time of the rebuild itself.
  This restores each scenario's *current* status, not its transition history;
  that history only survives if `ft.db` itself does. Entries whose `id` has no
  matching row (see below) are skipped — not an error.
  3. Re-run the existing test-link scan (`syncTestLinks`) to rebuild
  `test_links` — this already works from source, no change needed.

  **The two directions are not symmetric, and that asymmetry matters.** An
  earlier version of this design gated *both* directions on "the other side
  is completely empty" — replay only when the DB has nothing, backfill only
  when the file has nothing. That was wrong for the DB → file direction:
  real projects almost always have *some* rows in the file already (from
  ordinary `ft status` use), so "file is empty" essentially never held after
  the first status was ever set, and status history that predated
  `statuses.csv` (an existing project adopting this feature) silently never
  got backfilled — confirmed against this project's own repo, which had 207
  scenarios with DB status history but only 13 rows in the file, unnoticed
  until real use surfaced it.

  The fix: **DB → file runs unconditionally, on every sync** — the file is
  fully rewritten from each scenario's current status in the DB every time,
  so it's always an accurate snapshot regardless of what it already
  contained. If nothing changed, the rewrite produces identical bytes, so
  this doesn't create diff noise on an unchanged project. **File → DB stays
  gated on "DB has zero status rows,"** because that direction is genuinely
  a one-time, exceptional event (fts/ft.db was lost) — running it whenever
  the file simply has rows the DB doesn't (which, per above, is not a
  meaningful signal) would risk inserting duplicate or conflicting history
  into an otherwise-healthy DB.

- **Orphaned status history — resolved: skip, don't reconstruct.** A scenario
that was removed from its `.ft` file but had status history is kept in the DB
today (soft-delete: `removed` status, row retained) per
[FILE_CHANGES.md](FILE_CHANGES.md). Its `id` will still appear in
`statuses.csv` after that removal (the existing `removed` row already
reserves the id against reuse on rebuild — no separate mechanism needed for
that), but **its name and content are nowhere on disk** once the `Scenario:`
block is gone from the file — that information genuinely doesn't exist outside
`ft.db`.

  Rather than reconstructing a stand-in for it, the rebuild simply doesn't
  recreate that scenario: replay skips any `statuses.csv` row whose `id` has
  no corresponding `scenarios` row once the file-parsing pass is done, leaving
  no `statuses` row and no `scenarios` row for it at all. The row stays in
  `statuses.csv` untouched, so nothing is lost from the file itself, only from
  what gets replayed into the fresh DB.

  This is only actually at risk if `fts/ft.db` is deleted (or lost) — the
  ordinary case, where the DB is intact, is completely unaffected, since none
  of this rebuild logic runs. Losing an already-removed scenario's status in
  that rare case is an acceptable tradeoff against the complexity of
  reconstructing placeholder rows (and updating `ft show`/`ft list` to render
  scenarios with no name or content) for something that, by definition, no
  longer exists in any tracked file.

  (A scenario with **no** status history at all is hard-deleted via
  `store.DeleteScenario` without ever touching `Store.InsertStatus`, so it
  leaves no trace in `statuses.csv` in the first place — same outcome, for the
  same underlying reason: nothing else in the system references it.)

- **`ft init`.** Needs only the new "create `statuses.csv` if absent" step —
no routing logic, no awareness of rebuild vs. fresh project. That distinction
is `ft sync`'s to make, via the DB-has-zero-status-rows check above.

- **`.gitignore` handling (`ensureGitignore`).** No change needed for `ft.db`'s
entry, but nothing should ever add `statuses.csv` to it.

- **Daemon (`ftd`), when built.** Any status-changing code path in the daemon
must go through the same `InsertStatus` choke point (or an equivalent shared
helper) so it can't write to the DB without also upserting into the file.

- **Concurrent writers.** Because a write is now a read-modify-write (to find
and replace an existing id's row, or insert a new one in sorted position)
rather than a pure `O_APPEND`, it is **not** safe against two writers touching
the file at the same moment — the second writer's read can miss the first
writer's not-yet-flushed change, and whichever write lands last wins,
silently dropping the other. Today this is moot: only the CLI writes to this
file, one process at a time. It becomes a real question once the daemon
(Phase 12) exists alongside the CLI — they'll need to coordinate around this
file the same way they already coordinate around the DB (via WAL and the
daemon-pause-on-`ft sync` behavior in [DESIGN.md](DESIGN.md)), or accept the
occasional lost update as an acceptable risk for a best-effort snapshot file.
Not a blocker for this phase, but worth flagging now rather than discovering
it later.
