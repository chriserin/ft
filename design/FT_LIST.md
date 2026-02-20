# `ft list`

List all tracked scenarios as a flat list.

## Output Format

```
@ft:1  login.ft      User logs in              accepted
@ft:2  login.ft      User fails to log in      no-activity
@ft:3  checkout.ft   User completes purchase   in-progress
@ft:4  checkout.ft   User cancels order        done
```

Columns:
- `@ft:<id>` — scenario ID
- File name — the `.ft` file the scenario belongs to
- Scenario name — parsed from `Scenario:` line
- Current status — most recent status, or `no-activity` if no status records exist

## Filtering

```
ft list --status=<status>
ft list --no-activity
```

- `--status=<status>` filters scenarios by their current status
- `--no-activity` shows only scenarios with no status records
- If no scenarios match the filter, the output is empty (no error)

## Sort Order

Default sort is by file path, then by scenario ID.

## Deleted Files

Scenarios belonging to deleted files are not shown.
