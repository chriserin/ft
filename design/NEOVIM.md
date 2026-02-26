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

### Status History

Show the status history of the scenario under the cursor in a vertical split.

```
History: @ft:1 User logs in
  modified     Feb 25, 2026 3:12pm
  accepted     Feb 24, 2026 7:20pm
  ready        Feb 23, 2026 10:17pm
```

- Triggered by `gd` in `.ft` files (buffer-local keymap)
- Find scenario under cursor using `find_scenario_at_cursor`
- Call `ft show --history <id>` async
- Open a vertical split with a scratch buffer (`buftype = "nofile"`, `modifiable = false`)
- First line shows `History: @ft:<id> <scenario name>`, followed by the status history rows
- Buffer is cleaned up with `BufLeave` autocmd — closing the split deletes the buffer
- If no `@ft` tag found under cursor, notify user to run `:FtSync`

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

### Test Navigation

Jump between scenarios in `.ft` files and their linked tests in source code.

#### Scenario → Test (`gt`)

From a `.ft` file, jump to the test(s) linked to the scenario under the cursor.

```
  @ft:42
  Scenario: User logs in       ← cursor here
    Given a registered user
    When  they log in
    Then  they see the dashboard
```

- Triggered by `gt` in `.ft` files (buffer-local keymap)
- Find scenario under cursor using `find_scenario_at_cursor`
- Call `ft tests <id>` async
- **One test:** jump directly to `file:line`
- **Multiple tests:** populate quickfix list, open with `copen`
- **No tests:** `vim.notify("no linked tests for @ft:<id>")`

#### Test → Scenario (`gT`)

From a test file, jump to the scenario referenced by the `@ft:<id>` tag near the cursor.

```go
// @ft:42                      ← cursor here or on next line
func TestUserLogsIn(t *testing.T) {
```

- Triggered by `gT` in test files (buffer-local keymap, registered via autocommand on `BufEnter *_test.go`)
- Scan the current line and the line above for `@ft:(\d+)`
- Locate the `@ft:<id>` tag in the `fts/` directory using `vim.fn.search` across `.ft` files
- Jump to the tag line in the `.ft` file

#### CLI Integration

New `cli.lua` function:

```lua
function M.tests(cwd, id, callback)
  run({ "tests", tostring(id) }, cwd, callback)
end
```

New `parse.lua` function to parse `ft tests` output:

```lua
function M.parse_tests_output(stdout)
  local results = {}
  for line in stdout:gmatch("[^\n]+") do
    local file, lnum = line:match("^%s*(.+):(%d+)%s*$")
    if file and lnum then
      table.insert(results, { file = file, lnum = tonumber(lnum) })
    end
  end
  return results
end
```

#### Keymap Configuration

```lua
keymaps = {
  mappings = {
    ["gt"]         = "goto_test",       -- scenario → test (in .ft files)
    ["gT"]         = "goto_scenario",   -- test → scenario (in test files)
  },
},
```

### Tested Virtual Text

Display a `tested` indicator to the right of the status virtual text on `@ft:<id>` lines. Shows at a glance which scenarios have linked tests.

```
  @ft:1 accepted tested
  Scenario: User logs in
    Given a registered user

  @ft:2 ready
  Scenario: User fails login
    Given a registered user
```

#### Implementation

The extmark API supports multiple chunks in `virt_text`, each with its own highlight group:

```lua
virt_text = {
  { " accepted", "DiagnosticOk" },
  { " tested", "DiagnosticHint" },
}
```

During `virtual_text.refresh()`:

1. Fetch all scenarios with `cli.list(cwd, nil, ...)` — builds `status_map`
2. Fetch tested scenarios with `cli.list(cwd, {"tested"}, ...)` — builds `tested_set`
3. For each `@ft:<id>` tag in the buffer:
   - Start `virt_text` with the status chunk: `{ " " .. status, hl }`
   - If the ID is in `tested_set`, append: `{ " tested", tested_hl }`

Both `cli.list` calls run in parallel (both use async `vim.system`). The extmarks are rendered once both callbacks have fired.

#### Highlight Group

| Indicator | Highlight Group   | Typical Color |
| --------- | ----------------- | ------------- |
| `tested`  | `DiagnosticHint`  | cyan/teal     |

Added to the config:

```lua
virtual_text = {
  tested_hl = "DiagnosticHint",    -- highlight for "tested" indicator
},
```

#### Parallel Fetch Pattern

```lua
function M.refresh(bufnr)
  local cwd = cli.find_root_for_buffer(bufnr)
  if not cwd then return end

  local status_map, tested_set
  local pending = 2

  local function try_render()
    pending = pending - 1
    if pending > 0 then return end
    -- both calls complete — render extmarks
    render(bufnr, status_map, tested_set)
  end

  cli.list(cwd, nil, function(err, scenarios)
    status_map = {}
    if not err and scenarios then
      for _, s in ipairs(scenarios) do
        status_map[s.id] = s.status
      end
    end
    try_render()
  end)

  cli.list(cwd, { "tested" }, function(err, scenarios)
    tested_set = {}
    if not err and scenarios then
      for _, s in ipairs(scenarios) do
        tested_set[s.id] = true
      end
    end
    try_render()
  end)
end
```

### Autocommands

Augroup `FtNvim`:

- `BufEnter *.ft` — run `ft sync`, reload buffer if changed, refresh virtual text
- `BufWritePost *.ft` — run `ft sync`, reload buffer, refresh virtual text
- `FileType ft` — register buffer-local status keymaps
- `BufEnter *_test.go` — register buffer-local `gT` keymap for test → scenario navigation

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
    tested_hl = "DiagnosticHint",   -- highlight for "tested" indicator
    position = "eol",           -- "eol" or "right_align"
  },

  keymaps = {
    enabled = true,
    mappings = {                -- key -> status (buffer-local in .ft files)
      ["<leader>tr"] = "ready",
      ["<leader>ta"] = "accepted",
      ["<leader>ff"] = "find",      -- open scenario picker
      ["gt"]         = "goto_test",      -- scenario → linked test
      ["gT"]         = "goto_scenario",  -- test → scenario definition
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

Exported functions: `list()`, `set_status()`, `sync()`, `tests()`, `find_root_for_buffer()`

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
