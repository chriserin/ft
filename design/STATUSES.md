# `ft` — Statuses

Status is tracked per scenario. Files do not have their own status.

Every status change is recorded in the `statuses` table with a timestamp. There are no enforced transitions — a scenario can move from any status to any other status.

A scenario with no status records has not been worked on in any meaningful way.

Valid statuses: `accepted`, `in-progress`, `done`, `blocked`, `removed`

`removed` is set by the system when a scenario is deleted from a file but has existing status history.
