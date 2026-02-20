# `ft show`

Display a single scenario by its `@ft:<id>`.

## Output Format

```
@ft:42  login.ft
Status: accepted

History:
  accepted     2026-02-15 10:30

Tests:
  tests/test_login.py:14
  tests/test_auth.js:27

Scenario: User logs in
  Given the user is on the login page
  When the user enters valid credentials
  Then the user sees the dashboard
```

Sections:
- **Header** — scenario ID, file name
- **Status** — current status (most recent from `statuses` table), or `no-activity` if none
- **History** — all status records, most recent first, with timestamps
- **Tests** — linked test files and line numbers
- **Content** — the full gherkin content read from the `.ft` file on disk

Status and History sections are only shown once statuses exist (Phase 6).
Tests section is only shown once test links exist (Phase 8).

## Deleted Files

If the scenario's file has been deleted, the file is recreated from stored content before displaying (see DESIGN.md).

## Errors

- Unknown `<id>` — error message, no output
