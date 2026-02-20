# `ft show`

Display a single scenario by its `@ft:<id>`.

## Output Format

```
@ft:42  login.ft
Status: accepted

History:
  accepted     Feb 15, 2026 10:30am

Tests:
  tests/test_login.py:14
  tests/test_auth.js:27

Background:
  Given a registered user

Scenario: User logs in
  Given the user is on the login page
  When the user enters valid credentials
  Then the user sees the dashboard
```

Sections:
- **Header** — scenario ID, file name
- **Status** — current status (most recent from `statuses` table), or `no-activity` if none
- **History** — all status records, most recent first, with human-readable timestamps (`Jan 2, 2026 3:04pm`)
- **Tests** — linked test files and line numbers
- **Content** — the full gherkin content read from the `.ft` file on disk. If the file has a `Background:` section, it is shown before the scenario content to provide full context. If no Background exists, the content starts with the `Scenario:` line.

Status and History sections are only shown once statuses exist (Phase 6).
Tests section is only shown once test links exist (Phase 8).

## Colors

| Element              | Style                | Notes                                       |
|----------------------|----------------------|---------------------------------------------|
| `@ft:<id>`           | blue (4)             | Reuses `idStyle` from list                  |
| File name            | dim                  | Secondary info, doesn't compete with ID     |
| `Status:` label      | plain                | Uncolored label                             |
| Status value         | dim                  | Reuses `trkStyle`; will evolve per-status   |
| `Background:`        | bold                 | Section keyword                             |
| `Scenario:`          | bold                 | Section keyword                             |
| Step keywords        | cyan (6)             | Given, When, Then, And, But                 |
| Step text            | plain                | Default terminal color                      |
| Doc string delimiters| dim                  | `"""` or ` ``` ` lines                      |
| Doc string content   | dim                  | Content between doc string delimiters       |

The color applies to the keyword only, not the full line. Step keywords are colored wherever they appear as the first word of a step line (after indentation). Doc strings are rendered entirely in dim to visually distinguish them from step text.

## Deleted Files

If the scenario's file has been deleted, the file is recreated from stored content before displaying (see DESIGN.md).

## Errors

- Unknown `<id>` — error message, no output
