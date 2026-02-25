# `modified` Status

When `ft sync` detects that a scenario's content has changed, it automatically sets the scenario's status to `modified`.

## Behavior

During reconciliation, sync already compares the stored content against the parsed file content. When content has changed, sync currently updates the database row but does not insert a status record. With this change, sync inserts a `modified` status record.

## Rules

- Only insert `modified` if the scenario's latest status is **not** already `modified` (avoid duplicates on repeated syncs without edits)
- A scenario name change alone does **not** trigger `modified` — only the step content matters
- Restored scenarios (previously `removed`) do **not** trigger `modified` — they already get a `restored` status
- Name-matched scenarios (tag was missing, matched by name) do **not** trigger `modified` — these are re-linked, not content-edited

## Status Lifecycle

A typical scenario lifecycle with `modified`:

```
no-activity → accepted → modified → accepted → modified → done
```

The `modified` status signals that a scenario's content has changed since its last manual status update. The team can review the change and set the status back to `accepted`, `in-progress`, or whatever is appropriate.

## Implementation

In `reconcileTrackedFile`, at the point where `nameChanged || contentChanged` is true and the scenario was not restored, insert a status record:

```go
} else if nameChanged || contentChanged {
    sqlDB.Exec(`UPDATE scenarios SET ...`)
    if contentChanged && !scenarioLatestStatusIsModified(sqlDB, tagID) {
        sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, 'modified')`, tagID)
    }
    actions = append(actions, scenarioAction{kind: "modified", ...})
}
```
