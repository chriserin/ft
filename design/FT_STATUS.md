# `ft status`

## Without arguments — Project Report

Display a high-level summary of the project.

```
ft status
```

```
Scenarios: 12

  accepted      3
  in-progress   4
  done          2
  blocked       1
  no-activity   2
```

- Total scenario count
- Counts grouped by current status
- `no-activity` count for scenarios with no status records
- Statuses are listed in order of occurrence (most common first), with `no-activity` always last

Only statuses with a non-zero count are shown. If no scenarios have a given status, it is omitted from the list.

## With arguments — Update Scenario Status

```
ft status <id> <status>
```

- Insert a new `statuses` record for the scenario by its `@ft:<id>`
- Accept any text as status
- Print confirmation: `@ft:<id> → <status>`
- If the scenario's file is deleted, recreate the file from stored content before proceeding
