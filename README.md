# ft

AI-first feature tracking for personal projects

- [Overview](#overview)
- [Installation](#installation)
- [Getting Started](#getting-started)
- [Commands](#commands)
- [Workflow](#workflow)
- [Neovim Integration](#neovim-integration)

## Overview

`ft` manages `.ft` plain-text gherkin files. Generate these files with your
AI, track the scenarios contained within in neovim, and ensure that these
scenarios are tested by linking the scenarios to your tests.

## Installation

```bash
go install github.com/chriserin/ft@latest
```

With [mise](https://mise.jdx.dev):

```bash
mise use -g go:github.com/chriserin/ft
```

Or build from source:

```bash
git clone https://github.com/chriserin/ft.git
cd ft
go build -o ft .
```

## Commands

| Command     | Usage                       | Description                                                                                              |
| ----------- | --------------------------- | -------------------------------------------------------------------------------------------------------- |
| `ft init`   | `ft init`                   | Initialize ft in the current directory, creating `fts/` and the SQLite database                          |
| `ft sync`   | `ft sync`                   | Scan `fts/` for `.ft` files, register scenarios, and link tests via `@ft:<id>` tags                      |
| `ft list`   | `ft list [status...]`       | List all tracked scenarios, optionally filtered by status (`--not` to exclude)                           |
| `ft show`   | `ft show <id>`              | Show scenario details including content, status history, and linked tests (`--history` for history only) |
| `ft status` | `ft status [<id> <status>]` | Show project status summary, or update a scenario's status                                               |
| `ft tests`  | `ft tests <id>`             | List test files linked to a scenario                                                                     |

## Workflow

### 1. Initialize

```bash
cd my-project
ft init
```

This creates the `fts/` directory and a SQLite database to track scenarios.

### 2. Write feature files

Create `.ft` files in `fts/` using your AI or by hand. Write scenarios in Gherkin format:

```gherkin
Feature: User Login
  Users can log in with their credentials.

  Scenario: Valid login
    Given a registered user
    When they log in with valid credentials
    Then they see the dashboard
```

### 3. Sync

```bash
ft sync
```

Sync parses your `.ft` files, registers each scenario in the database, and writes `@ft:<id>` tags above each scenario. It also scans `_test.go` files for `@ft:<id>` comments to discover test links.

```
  new  fts/login.ft
       + @ft:1 Valid login
       + @ft:2 Invalid login

synced 1 files, 2 scenarios
```

### 4. Track progress

Update scenario statuses as you work:

```bash
ft status 1 ready # when a scenario is ready to be implemented
ft status 1 in-progress # as implementation is begun
ft status 1 fulfilled # as implementation is complete
ft status 1 accepted # as the scenario has been confirmed
```

Check overall project status:

```bash
ft status
```

Which Outputs:

```
Scenarios: 191
  accepted: 175
  removed: 15
  modified: 1
```

### 5. Link tests

[Warning] Currently only works for go files!

Add `@ft:<id>` comments above test functions to link them to scenarios:

```go
// @ft:1
func TestValidLogin(t *testing.T) {
    // ...
}
```

Run `ft sync` again to register the links. Then filter by test coverage:

```bash
ft list tested         # scenarios with linked tests
ft list --not tested   # scenarios still needing tests
```

## Neovim Integration

The [ft.nvim](https://github.com/chriserin/ft.nvim) plugin provides inline status tracking, navigation, and scenario management directly in Neovim.

### Virtual Text

Scenario statuses appear as colored inline text next to `@ft:<id>` tags. A `tested` badge is shown for scenarios with linked tests.

```
  @ft:1 accepted tested
  Scenario: User logs in
    Given a registered user
    When they enter valid credentials
    Then they see the dashboard

  @ft:2 in-progress
  Scenario: User logs out
```

### Keymaps

| Keymap       | Context          | Action                                           |
| ------------ | ---------------- | ------------------------------------------------ |
| `<leader>tr` | `.ft` files      | Mark scenario under cursor as "ready"            |
| `<leader>ta` | `.ft` files      | Mark scenario under cursor as "accepted"         |
| `<leader>ff` | any file         | Open scenario finder/picker                      |
| `gt`         | `.ft` files      | Jump to linked test(s) for scenario under cursor |
| `gT`         | `_test.go` files | Jump from test to its scenario definition        |
| `gd`         | `.ft` files      | Show status history in a vertical split          |

### Commands

| Command                   | Description                                                       |
| ------------------------- | ----------------------------------------------------------------- |
| `:FtSync`                 | Run `ft sync`, reload buffer, and refresh virtual text            |
| `:FtFind`                 | Open scenario picker to browse and jump to scenarios              |
| `:FtList [status...]`     | Open quickfix list filtered by status (prefix with `!` to negate) |
| `:FtStatus <id> <status>` | Set a scenario's status by ID                                     |

### Auto-Sync

The plugin automatically runs `ft sync` when opening or saving `.ft` files, keeping the database and virtual text in sync without manual intervention.
