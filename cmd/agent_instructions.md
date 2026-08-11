# ft — Agent Instructions

`ft` tracks feature scenarios in Gherkin `.ft` files under `fts/`. Use the `ft` CLI to read and update scenario state — never edit `fts/ft.db` or `fts/statuses.csv` directly.

## Core Concepts

- A scenario is a single `Scenario:` block in a `.ft` file. @ft:<id> tags are permanent and owned by `ft sync` — never edit or renumber a tag yourself.

## Commands

- `ft list [status...]` — list scenarios, optionally filtered by status.
- `ft show <id>` — show a scenario's content, current status, history, and linked tests.
- `ft status` — project-wide status counts.
- `ft status <id> <status>` — set a scenario's status.

## Statuses

Three sets, by who is allowed to set each status. Enforced by convention, not by tool.

### System-set — never set these via `ft status`, agent or human

- `modified`
- `removed`
- `restored`

### Human-set — the agent must not set any of these

- `ready` — a human has reviewed the scenario's spec and judged ready to implement.
- `accepted` — a human has confirmed `fulfilled` work is actually correct.
- `rejected` — a human reviewed a story and found it incorrect by definition or by implementation.

### Agent-set

- `in-progress` — implementation has started on a `ready` scenario.
- `fulfilled` — a scenario is implemented, a test exists, is linked (`@ft:<id>` above the test function), and passes.

## Workflow

Only start on scenarios already marked `ready`. Mark `in-progress`, implement with a linked test, and verify the test passes before marking `fulfilled`. Stop there.
