# `ft` — File Change Handling

When the daemon detects a change to a `.ft` file, it re-parses the file and
reconciles the result against the database. This document describes the
behavior for each type of file event.

---

## File Created

A new `.ft` file appears on disk.

1. Parse the file to extract all `Scenario:` blocks and any existing `@ft:` tags
2. If any scenarios have `@ft:` tags that match existing DB records, this is a **renamed/moved file**:
   - Update the `file_path` on the existing `files` record
   - Process remaining scenarios as in **File Modified** (match tagged, insert untagged)
3. Otherwise, this is a truly new file:
   a. Insert a new `files` record with the file path and name
   b. For each scenario, insert a `scenarios` record, obtaining its `id`, and write an `@ft:<id>` tag to the file

## File Modified

An existing tracked `.ft` file is changed on disk.

1. Re-parse the file to extract all `Scenario:` blocks and their `@ft:` tags
2. Match scenarios between file and DB by `@ft:<id>` tag
   - **Tagged scenario found in DB** — update name, content, and `updated_at` timestamp. Status history is retained
   - **Tagged scenario with unknown ID** — fall back to matching by scenario name within the same file. If a name match is found, re-associate and write the correct `@ft:<id>` tag. If no name match, treat as a new scenario.
   - **Untagged scenario** — fall back to matching by scenario name within the same file. If a name match is found, re-associate and write the `@ft:<id>` tag. If no name match, new scenario; insert a `scenarios` record (with content), write `@ft:<id>` tag to the file
   - **Tag in DB but not in file** — scenario was removed:
     - If the scenario has active test links — **rehydrate**: write the scenario back to the file using the stored content and its `@ft:<id>` tag. This removal was a mistake; tests still reference this scenario.
     - If the scenario has status history but no test links — insert a `removed` status record
     - If the scenario has no status history and no test links — delete the scenario row

Because matching is by DB `id` tag, a scenario can be freely renamed without losing its status history.

## File Deleted

A tracked `.ft` file is removed from disk.

1. For each scenario belonging to this file:
   - If the scenario has active test links — **rehydrate**: recreate the file and write the scenario back using the stored content and its `@ft:<id>` tag. Tests still reference this scenario, so the file cannot be fully deleted.
   - If the scenario has status history but no test links — insert a `removed` status record
   - If the scenario has no status history and no test links — delete the scenario row
2. If any scenarios were rehydrated, the file is restored (not deleted). Write an error comment to the top of the file indicating which scenarios were preserved because of active test links.
3. If no scenarios were rehydrated, set `deleted = TRUE` on the `files` record.

The file record is kept for referential integrity — scenarios with status history still reference it. The last known scenario content is preserved in the `content` column from the most recent parse.

## File Renamed / Moved

File renames are not detected as a distinct event. Most file watchers report a rename as a delete followed by a create. The **File Created** logic handles this by checking for existing `@ft:` tags — if the "new" file contains tags that match DB records, it's recognized as a rename and the file path is updated.

## Startup Reconciliation

When the daemon starts, it performs a full sync:

1. Scan the directory tree for all `.ft` files
2. For each file found on disk:
   - If tracked in the DB — treat as a **file modified** event (re-parse and
     diff)
   - If not tracked — treat as a **file created** event
3. For each file tracked in the DB but missing from disk (excluding files already marked `deleted`):
   - Treat as a **file deleted** event
