# `ft.nvim` — Implementation

Lua plugin for Neovim 0.11.6+. All data access goes through the `ft` CLI binary.

---

## Plugin Structure

```
ft.nvim/
  lua/ft/
    init.lua          -- setup(), user commands, filetype registration
    config.lua        -- config schema with defaults
    cli.lua           -- async ft CLI wrapper
    parse.lua         -- buffer pattern matching + CLI output parsing
    virtual_text.lua  -- extmark rendering
    telescope.lua     -- Telescope picker + vim.ui.select fallback
    status.lua        -- cursor-context status resolution
    autocmds.lua      -- autocommand registration
    health.lua        -- :checkhealth ft
  plugin/
    ft.vim            -- double-load guard
```

---

## Build Order

Each step produces a testable unit. Later modules depend on earlier ones.

### Step 1: `plugin/ft.vim`

Guard against double-loading:

```vim
if exists('g:loaded_ft_nvim')
  finish
endif
let g:loaded_ft_nvim = 1
```

### Step 2: `config.lua`

Pure data module. Stores defaults and merged user config.

```lua
local M = {}
local _config = nil

local defaults = {
  bin = nil,

  virtual_text = {
    enabled = true,
    hl = {
      ["accepted"]    = "DiagnosticOk",
      ["in-progress"] = "DiagnosticWarn",
      ["done"]        = "DiagnosticWarn",
      ["ready"]       = "DiagnosticOk",
    },
    hl_default = "Comment",
    position = "eol",
  },

  keymaps = {
    enabled = true,
    mappings = {
      ["<leader>tr"] = "ready",
      ["<leader>ta"] = "accepted",
      ["<leader>ff"] = "list",
    },
  },

  sync_on_write = true,
  telescope = { enabled = true },
}

function M.setup(opts)
  _config = vim.tbl_deep_extend("force", defaults, opts or {})
end

function M.get()
  if not _config then
    M.setup({})
  end
  return _config
end

return M
```

No dependencies. Testable by calling `setup()` and inspecting `get()`.

### Step 3: `parse.lua`

Pure Lua functions for pattern matching. No Neovim API calls, no CLI calls.

**`find_ft_tags(bufnr)`** — scan buffer lines, return `{ [line_0indexed] = id, ... }`:

```lua
local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
for i, line in ipairs(lines) do
  local id = line:match("^%s*@ft:(%d+)%s*$")
  if id then
    result[i - 1] = tonumber(id)
  end
end
```

**`find_scenario_at_cursor(bufnr)`** — walk up from cursor to find the nearest `@ft:<id>` tag:

```lua
function M.find_scenario_at_cursor(bufnr)
  local cursor = vim.api.nvim_win_get_cursor(0)
  local row = cursor[1] -- 1-based
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, row, false)

  for i = #lines, 1, -1 do
    local id = lines[i]:match("^%s*@ft:(%d+)%s*$")
    if id then
      return tonumber(id)
    end
    -- Stop at Feature: line
    if lines[i]:match("^%s*Feature:") then
      return nil
    end
  end
  return nil
end
```

**`parse_list_output(stdout)`** — parse `ft list` output into a table:

The output is column-aligned with space padding. `@ft:<id>` is the first token, the filename is the next token (no spaces in filenames), the status is the last token (single word). Everything between the filename and status is the scenario name.

```lua
function M.parse_list_output(stdout)
  local results = {}
  for line in stdout:gmatch("[^\n]+") do
    local id = line:match("^@ft:(%d+)")
    if id then
      local rest = line:match("^@ft:%d+%s+(.*)")
      local file = rest:match("^(%S+)")
      rest = rest:match("^%S+%s+(.*)")
      local name, status = rest:match("^(.-)%s%s+(%S+)%s*$")
      table.insert(results, {
        id = tonumber(id),
        file = file,
        name = name,
        status = status,
      })
    end
  end
  return results
end
```

### Step 4: `cli.lua`

Async wrapper around the `ft` binary using `vim.system`.

**`find_root(start_path)`** — walk up directories looking for `fts/`:

```lua
local function find_root(start_path)
  local path = start_path
  while path and path ~= "/" do
    if vim.fn.isdirectory(path .. "/fts") == 1 then
      return path
    end
    path = vim.fn.fnamemodify(path, ":h")
  end
  return nil
end
```

**`run(cmd_args, cwd, callback)`** — core async executor:

```lua
local function run(cmd_args, cwd, callback)
  local bin = config.get().bin or "ft"
  local cmd = vim.list_extend({ bin }, cmd_args)

  vim.system(cmd, { cwd = cwd, text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        callback(result.stderr or ("ft exited with code " .. result.code), nil)
      else
        callback(nil, result.stdout)
      end
    end)
  end)
end
```

Every callback is wrapped in `vim.schedule` because `vim.system` callbacks run on a libuv thread, and Neovim API calls must happen on the main loop.

**Exported functions:**

```lua
function M.find_root_for_buffer(bufnr)
  local path = vim.api.nvim_buf_get_name(bufnr)
  return find_root(vim.fn.fnamemodify(path, ":h"))
end

function M.sync(cwd, callback)
  run({ "sync" }, cwd, callback)
end

function M.list(cwd, callback)
  run({ "list" }, cwd, function(err, stdout)
    if err then return callback(err, nil) end
    callback(nil, require("ft.parse").parse_list_output(stdout))
  end)
end

function M.set_status(cwd, id, status, callback)
  run({ "status", tostring(id), status }, cwd, callback)
end
```

### Step 5: `virtual_text.lua`

Renders scenario statuses as virtual text on `@ft:<id>` lines using extmarks.

```lua
local ns = vim.api.nvim_create_namespace("ft_virtual_text")

function M.refresh(bufnr)
  if not config.get().virtual_text.enabled then return end
  if not vim.api.nvim_buf_is_valid(bufnr) then return end

  local cwd = cli.find_root_for_buffer(bufnr)
  if not cwd then return end

  cli.list(cwd, function(err, scenarios)
    if err or not scenarios then return end
    if not vim.api.nvim_buf_is_valid(bufnr) then return end

    -- Build id -> status lookup
    local status_map = {}
    for _, s in ipairs(scenarios) do
      status_map[s.id] = s.status
    end

    -- Find tags in buffer
    local tags = parse.find_ft_tags(bufnr)

    -- Clear existing extmarks
    vim.api.nvim_buf_clear_namespace(bufnr, ns, 0, -1)

    -- Render
    local vt_config = config.get().virtual_text
    for line, id in pairs(tags) do
      local status = status_map[id]
      if status then
        local hl = vt_config.hl[status] or vt_config.hl_default
        vim.api.nvim_buf_set_extmark(bufnr, ns, line, 0, {
          virt_text = { { " " .. status, hl } },
          virt_text_pos = vt_config.position,
          hl_mode = "combine",
        })
      end
    end
  end)
end
```

Guard `nvim_buf_is_valid` both before the async call and inside the callback — the buffer may close while `ft list` is running.

### Step 6: `status.lua`

Resolves the scenario under the cursor and calls `ft status`.

```lua
function M.set_status_under_cursor(status_name)
  local bufnr = vim.api.nvim_get_current_buf()
  local id = parse.find_scenario_at_cursor(bufnr)

  if not id then
    vim.notify("No @ft tag found — run :FtSync", vim.log.levels.WARN)
    return
  end

  local cwd = cli.find_root_for_buffer(bufnr)
  if not cwd then
    vim.notify("Not in an ft project", vim.log.levels.ERROR)
    return
  end

  cli.set_status(cwd, id, status_name, function(err, output)
    if err then
      vim.notify("ft status failed: " .. err, vim.log.levels.ERROR)
      return
    end
    vim.notify(vim.trim(output), vim.log.levels.INFO)
    virtual_text.refresh(bufnr)
  end)
end
```

The `ft status` CLI outputs the confirmation line (e.g. `@ft:1 pass → accepted`), which is passed directly to `vim.notify`.

### Step 7: `autocmds.lua`

Creates the `FtNvim` augroup with all autocommands.

```lua
function M.setup()
  local cfg = config.get()
  local group = vim.api.nvim_create_augroup("FtNvim", { clear = true })

  -- BufEnter: sync + refresh virtual text
  vim.api.nvim_create_autocmd("BufEnter", {
    group = group,
    pattern = "*.ft",
    callback = function(args)
      local cwd = cli.find_root_for_buffer(args.buf)
      if not cwd then return end
      cli.sync(cwd, function(err, _)
        if err then return end
        if not vim.api.nvim_buf_is_valid(args.buf) then return end
        -- Reload buffer if file changed on disk
        vim.api.nvim_buf_call(args.buf, function()
          vim.cmd("checktime")
        end)
        virtual_text.refresh(args.buf)
      end)
    end,
  })

  -- BufWritePost: sync + reload + refresh
  if cfg.sync_on_write then
    vim.api.nvim_create_autocmd("BufWritePost", {
      group = group,
      pattern = "*.ft",
      callback = function(args)
        local cwd = cli.find_root_for_buffer(args.buf)
        if not cwd then return end
        cli.sync(cwd, function(err, _)
          if err then return end
          if not vim.api.nvim_buf_is_valid(args.buf) then return end
          vim.api.nvim_buf_call(args.buf, function()
            vim.cmd("edit")
          end)
          virtual_text.refresh(args.buf)
        end)
      end,
    })
  end

  -- FileType: register buffer-local keymaps
  if cfg.keymaps.enabled then
    vim.api.nvim_create_autocmd("FileType", {
      group = group,
      pattern = "ft",
      callback = function(args)
        for key, action in pairs(cfg.keymaps.mappings) do
          if action == "list" then
            vim.keymap.set("n", key, function()
              require("ft.telescope").pick()
            end, { buffer = args.buf, desc = "ft: list scenarios" })
          else
            vim.keymap.set("n", key, function()
              require("ft.status").set_status_under_cursor(action)
            end, { buffer = args.buf, desc = "ft: " .. action })
          end
        end
      end,
    })
  end
end
```

`BufEnter` uses `checktime` to reload the buffer only if the file changed on disk (non-destructive). `BufWritePost` uses `edit` to force-reload after sync writes `@ft:<id>` tags.

### Step 8: `telescope.lua`

Telescope picker with `vim.ui.select` fallback.

**Telescope picker:**

```lua
function M.pick()
  local cwd = cli.find_root_for_buffer(0)
  if not cwd then
    vim.notify("Not in an ft project", vim.log.levels.ERROR)
    return
  end

  local ok, _ = pcall(require, "telescope")
  if not ok or not config.get().telescope.enabled then
    return M.fallback_pick(cwd)
  end

  cli.list(cwd, function(err, scenarios)
    if err then
      vim.notify(err, vim.log.levels.ERROR)
      return
    end

    local pickers = require("telescope.pickers")
    local finders = require("telescope.finders")
    local conf = require("telescope.config").values
    local actions = require("telescope.actions")
    local action_state = require("telescope.actions.state")
    local entry_display = require("telescope.pickers.entry_display")

    local displayer = entry_display.create({
      separator = "  ",
      items = {
        { width = 10 },
        { width = 30 },
        { remaining = true },
      },
    })

    pickers.new({}, {
      prompt_title = "ft scenarios",
      finder = finders.new_table({
        results = scenarios,
        entry_maker = function(entry)
          return {
            value = entry,
            display = function(e)
              return displayer({
                { "@ft:" .. e.value.id, "TelescopeResultsIdentifier" },
                { e.value.file, "TelescopeResultsComment" },
                { e.value.name .. "  " .. e.value.status },
              })
            end,
            ordinal = "@ft:" .. entry.id .. " " .. entry.file .. " "
              .. entry.name .. " " .. entry.status,
          }
        end,
      }),
      sorter = conf.generic_sorter({}),
      attach_mappings = function(prompt_bufnr, map)
        actions.select_default:replace(function()
          local selection = action_state.get_selected_entry()
          actions.close(prompt_bufnr)
          if selection then
            local s = selection.value
            vim.cmd("edit " .. s.file)
            vim.fn.search("@ft:" .. s.id, "w")
          end
        end)
        return true
      end,
    }):find()
  end)
end
```

**Fallback with `vim.ui.select`:**

```lua
function M.fallback_pick(cwd)
  cli.list(cwd, function(err, scenarios)
    if err then
      vim.notify(err, vim.log.levels.ERROR)
      return
    end
    vim.ui.select(scenarios, {
      prompt = "ft scenarios",
      format_item = function(s)
        return string.format("@ft:%-4d  %-20s  %s  [%s]", s.id, s.file, s.name, s.status)
      end,
    }, function(choice)
      if choice then
        vim.cmd("edit " .. choice.file)
        vim.fn.search("@ft:" .. choice.id, "w")
      end
    end)
  end)
end
```

### Step 9: `health.lua`

Neovim checkhealth integration. The module must export a `check()` function.

```lua
local M = {}

function M.check()
  vim.health.start("ft.nvim")

  -- Check ft binary
  local cfg = require("ft.config").get()
  local bin = cfg.bin or "ft"
  if vim.fn.executable(bin) == 1 then
    vim.health.ok("`" .. bin .. "` found")
  else
    vim.health.error("`" .. bin .. "` not found on PATH")
  end

  -- Check fts/ directory
  local cwd = require("ft.cli").find_root_for_buffer(0)
  if cwd then
    vim.health.ok("`fts/` directory found at " .. cwd)
  else
    vim.health.warn("`fts/` directory not found — run `ft init`")
  end

  -- Check fts/ft.db
  if cwd and vim.fn.filereadable(cwd .. "/fts/ft.db") == 1 then
    vim.health.ok("`fts/ft.db` exists")
  elseif cwd then
    vim.health.warn("`fts/ft.db` not found — run `ft init`")
  end

  -- Check Telescope
  local ok, _ = pcall(require, "telescope")
  if ok then
    vim.health.ok("telescope.nvim available")
  else
    vim.health.info("telescope.nvim not installed — :FtList will use vim.ui.select")
  end
end

return M
```

### Step 10: `init.lua`

Entry point. Ties everything together.

```lua
local M = {}

function M.setup(opts)
  require("ft.config").setup(opts)

  -- Register .ft filetype
  vim.filetype.add({ extension = { ft = "ft" } })

  -- User commands
  vim.api.nvim_create_user_command("FtSync", function()
    local cli = require("ft.cli")
    local cwd = cli.find_root_for_buffer(0)
    if not cwd then
      vim.notify("Not in an ft project", vim.log.levels.ERROR)
      return
    end
    cli.sync(cwd, function(err, output)
      if err then
        vim.notify(err, vim.log.levels.ERROR)
        return
      end
      vim.notify(vim.trim(output), vim.log.levels.INFO)
      local bufname = vim.api.nvim_buf_get_name(0)
      if bufname:match("%.ft$") then
        vim.cmd("edit")
        require("ft.virtual_text").refresh(0)
      end
    end)
  end, { desc = "Run ft sync" })

  vim.api.nvim_create_user_command("FtList", function()
    require("ft.telescope").pick()
  end, { desc = "List scenarios" })

  vim.api.nvim_create_user_command("FtStatus", function(opts)
    local args = vim.split(opts.args, " ", { trimempty = true })
    if #args < 2 then
      vim.notify("Usage: :FtStatus <id> <status>", vim.log.levels.ERROR)
      return
    end
    local cli = require("ft.cli")
    local cwd = cli.find_root_for_buffer(0)
    if not cwd then
      vim.notify("Not in an ft project", vim.log.levels.ERROR)
      return
    end
    local id = args[1]
    local status = table.concat(vim.list_slice(args, 2), " ")
    cli.set_status(cwd, id, status, function(err, output)
      if err then
        vim.notify(err, vim.log.levels.ERROR)
        return
      end
      vim.notify(vim.trim(output), vim.log.levels.INFO)
      local bufname = vim.api.nvim_buf_get_name(0)
      if bufname:match("%.ft$") then
        require("ft.virtual_text").refresh(0)
      end
    end)
  end, { nargs = "+", desc = "Set scenario status" })

  -- Autocommands
  require("ft.autocmds").setup()
end

return M
```

---

## Testing

Manual verification steps for each feature:

1. **Virtual text** — Open a synced `.ft` file, verify status text appears next to `@ft:<id>` lines with correct colors
2. **BufEnter sync** — Edit a `.ft` file outside Neovim, switch to it, verify new `@ft` tags appear and virtual text refreshes
3. **BufWritePost sync** — Add a new `Scenario:` block, save, verify `@ft` tag is written and virtual text appears
4. **Status keymaps** — Place cursor in a scenario, press `<leader>ta`, verify notification shows the transition and virtual text updates
5. **No-tag keymap** — Press a status keymap outside any scenario, verify warning notification
6. **Telescope** — Run `:FtList`, verify picker shows all scenarios, select one, verify jump to correct file and line
7. **Fallback** — Disable Telescope in config, run `:FtList`, verify `vim.ui.select` appears
8. **Checkhealth** — Run `:checkhealth ft`, verify binary, directory, database, and Telescope checks
9. **FtSync command** — Run `:FtSync`, verify notification with sync output
10. **FtStatus command** — Run `:FtStatus 1 accepted`, verify notification
