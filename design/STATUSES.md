# `ft` — Statuses

Status is tracked per scenario. Files do not have their own status.

Every status change is recorded in the `statuses` table with a timestamp. There are no enforced transitions — a scenario can move from any status to any other status.

A scenario with no status records has not been worked on in any meaningful way.

Valid statuses: `accepted`, `in-progress`, `done`, `blocked`, `removed`, `modified`

`removed` is set by the system when a scenario is deleted from a file but has existing status history.

`modified` is set by the system when `ft sync` detects that a scenario's step content has changed (see design/MOD.md). Name changes alone do not trigger `modified`.
