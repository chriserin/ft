# `ft.nvim` — Neovim Plugin

A Lua plugin for Neovim (0.11.6+) that integrates the `ft` CLI into the editor. Calls the `ft` binary for all data access — no direct SQLite queries.

Uses `Snacks.picker` for scenario browsing with preview.

---

## Features

### Virtual Text Status

Display scenario status inline next to `@ft:<id>` tags using extmarks.

```
  @ft:1 accepted
  Scenario: User logs in
    Given a registered user
    When  they enter valid credentials
    Then  they see the dashboard
```

- Scan buffer for lines matching `@ft:<id>`
- Run `ft list` async, build `id -> status` lookup
- Render with `nvim_buf_set_extmark` at end of line
- Color via `Diagnostic*` highlight groups — works with any color scheme
- Refresh on `BufEnter` and after status changes

#### Status Highlight Groups

| Status        | Highlight Group    | Typical Color |
| ------------- | ------------------ | ------------- |
| `accepted`    | `DiagnosticOk`     | green         |
| `ready`       | `DiagnosticOk`     | green         |
| `in-progress` | `DiagnosticWarn`   | yellow        |
| `fulfilled`   | `DiagnosticWarn`   | yellow        |
| `rejected`    | `DiagnosticError`  | red           |
| `modified`    | `DiagnosticInfo`   | blue          |
| (other)       | `Comment`          | gray          |

### Scenario Picker

Browse, filter, and jump to scenarios via `:FtFind`.

```
> user logs
  @ft:1 User logs in
  @ft:3 User logs out
  @ft:7 User logs payment details
```

- Parse `ft list` output into entries, excluding removed scenarios
- Fuzzy filter by `@ft:<id>` and scenario name
- Preview shows `ft show <id>` output for the selected scenario
- `<CR>` — open file, jump to `@ft:<id>` line
- Opens from any file in the project via `<leader>ff`

### Status Keymaps

Buffer-local key mappings in `.ft` files to set scenario status under the cursor.

- Walk up from cursor to find the nearest `Scenario:` line
- Read the `@ft:<id>` tag on the line above it
- Call `ft status <id> <status>` async
- Notify confirmation, refresh virtual text
- If no tag found, notify user to run `:FtSync`

### User Commands

| Command                   | Behavior                                                      |
| ------------------------- | ------------------------------------------------------------- |
| `:FtSync`                 | Run `ft sync`, notify output, reload buffer (tags may change) |
| `:FtFind`                 | Open scenario picker (Snacks.picker)                          |
| `:FtList <status...>`     | Open quickfix list of scenarios matching status filters. Multiple arguments supported. Prefix with `!` to negate (e.g. `:FtList !accepted`). Removed scenarios are always excluded. |

### Checkhealth

`:checkhealth ft` runs diagnostics:

- `ft` binary found on PATH (or configured `bin` path)
- `ft` binary version executes successfully
- `fts/` directory exists in the project root
- `fts/ft.db` database exists

### Autocommands

Augroup `FtNvim`:

- `BufEnter *.ft` — run `ft sync`, reload buffer if changed, refresh virtual text
- `BufWritePost *.ft` — run `ft sync`, reload buffer, refresh virtual text
- `FileType ft` — register buffer-local status keymaps

---

## Configuration

```lua
require("ft").setup({
  bin = nil,                    -- path to ft binary (default: "ft" from PATH)

  virtual_text = {
    enabled = true,
    hl = {                      -- status -> highlight group
      ["accepted"]    = "DiagnosticOk",
      ["in-progress"] = "DiagnosticWarn",
      ["fulfilled"]   = "DiagnosticWarn",
      ["ready"]       = "DiagnosticOk",
      ["rejected"]    = "DiagnosticError",
      ["modified"]    = "DiagnosticInfo",
    },
    hl_default = "Comment",
    position = "eol",           -- "eol" or "right_align"
  },

  keymaps = {
    enabled = true,
    mappings = {                -- key -> status (buffer-local in .ft files)
      ["<leader>tr"] = "ready",
      ["<leader>ta"] = "accepted",
      ["<leader>ff"] = "find",      -- open scenario picker
    },
  },

  sync_on_write = true,         -- run ft sync on BufWritePost
})

---

## Plugin Structure

```
ft.nvim/
  lua/ft/
    init.lua          -- setup(), user commands, filetype registration
    config.lua        -- config schema with defaults
    cli.lua           -- async ft CLI wrapper (vim.system)
    parse.lua         -- buffer pattern matching + CLI output parsing
    virtual_text.lua  -- extmark rendering
    picker.lua        -- Snacks.picker scenario browser
    status.lua        -- cursor-context status resolution
    autocmds.lua      -- autocommand registration
    health.lua        -- :checkhealth ft
  plugin/
    ft.vim            -- double-load guard
```

## CLI Wrapper

All `ft` calls go through `cli.lua` using `vim.system`:

```lua
vim.system(cmd, { cwd = root, text = true }, callback)
```

- Callbacks wrapped in `vim.schedule` (libuv thread -> main loop)
- Project root detected by walking up from buffer directory to find `fts/`
- lipgloss automatically strips ANSI when stdout is not a TTY

Exported functions: `list()`, `set_status()`, `sync()`, `find_root_for_buffer()`

## Parsing `ft list` Output

The output is column-aligned with space padding:

```
@ft:1   fts/login.ft      User logs in           accepted
@ft:12  fts/checkout.ft   User completes order   in-progress
```

Strategy: `@ft:<id>` from the left, status (last `%S+`) from the right, filename (first `%S+` after tag), everything between filename and status is the scenario name.

## Error Handling

- **Binary not found** — warn at setup, disable features
- **Not in project** — silent skip for autocmds, `vim.notify` for commands
- **CLI error** — surface stderr via `vim.notify(ERROR)`
- **Buffer gone** — guard with `nvim_buf_is_valid` in callbacks

---

## lazy.nvim Example

```lua
{
  "chriserin/ft.nvim",
  event = { "BufReadPre *.ft", "BufNewFile *.ft" },
  cmd = { "FtSync", "FtFind", "FtList" },
  opts = {
    keymaps = {
      mappings = {
        ["<leader>fa"] = "accepted",
        ["<leader>fr"] = "ready",
      },
    },
  },
}
```
