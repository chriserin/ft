# `ft` — Rehydration

Reconstructing `.ft` files from stored content in the database.

---

## When Rehydration Occurs

1. **Scenario removed from file but has active test links** — the scenario is written back into the file (see FILE_CHANGES.md)
2. **File deleted but scenarios have active test links** — the file is recreated with those scenarios
3. **CLI command accesses a scenario whose file is deleted** — the file is recreated before the command proceeds

## Data Sources

Rehydration uses two stored content fields:

- **`files.content`** — the file-level header: `Feature:` line, description, and `Background:` block
- **`scenarios.content`** — the full scenario block: `Scenario:` line, description, and steps (including doc strings and data tables)

Neither field includes `@ft:` tags — those are written separately during rehydration.

## File Reconstruction

### Full File (file deleted, all scenarios rehydrated)

```
<files.content>

@ft:<id1> <other tags>
<scenarios[0].content>

@ft:<id2> <other tags>
<scenarios[1].content>

...
```

1. Write the file-level content from `files.content` (Feature line, description, Background)
2. For each scenario to rehydrate:
   a. Write a blank line separator
   b. Write the tag line: `@ft:<id>` followed by any other tags the scenario had
   c. Write the scenario content from `scenarios.content`
3. Write the file to the original `file_path`
4. Clear `deleted = FALSE` on the `files` record

### Partial File (scenario removed but file still exists)

1. Read the current file from disk
2. Append the rehydrated scenario(s) to the end of the file:
   a. Write a blank line separator
   b. Write the tag line
   c. Write the scenario content
3. Write an error comment to the top of the file:
   ```
   # ft error: scenario(s) restored because active test links exist — remove tests before removing scenarios
   ```

## Edge Cases

### scenarios.content is empty or missing

If a scenario has no stored content (tracked before the `content` column was added):
- Write a minimal scenario block: `Scenario: <name>` with no steps
- The user will need to re-add the steps manually

### Multiple scenarios rehydrated into a deleted file

All scenarios belonging to the file that need rehydration are written at once. Scenarios are ordered by their database ID to preserve the original registration order.

### Scenario rehydrated but file was renamed

If the file at the original `file_path` no longer exists but the `files` record is not marked deleted (rename case), the rehydrated scenario is written to the current `file_path` on the `files` record.

## After Rehydration

- The rehydrated file is a valid `.ft` file and will be re-parsed on the next `ft sync`
- The re-parse will match scenarios by `@ft:` tag and update content in the DB
- Status history is unaffected — rehydration does not insert any status records
