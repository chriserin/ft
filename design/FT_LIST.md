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
ft list <status>
ft list --not <status>
ft list <status1> <status2>
ft list --not <status1> --not <status2>
ft list <status1> --not <status2>
```

- Positional arguments include scenarios matching that status
- `--not <status>` excludes scenarios matching that status (repeatable)
- Positive and negative filters can be mixed: `ft list ready --not no-activity` means "status is ready AND status is not no-activity"
- When only `--not` filters are given, all non-matching scenarios are shown: `ft list --not removed` shows everything except removed
- When only positive filters are given, only matching scenarios are shown: `ft list accepted ready` shows accepted and ready
- If no arguments are given, all scenarios are shown
- If no scenarios match the filter, the output is empty (no error)

## Sort Order

Default sort is by file path, then by scenario ID.

## Deleted Files

Scenarios belonging to deleted files are not shown.
