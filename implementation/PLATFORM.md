# `ft` — Platform

## Language

Go (latest stable version)

## Module

`github.com/chris/ft` (or appropriate module path)

## Dependencies

- **CLI framework**: [cobra](https://github.com/spf13/cobra) — subcommand-based CLI with auto-generated help
- **SQLite driver**: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGO required
- **Testing**: [testify](https://github.com/stretchr/testify) — assertions and test suites

## Project Structure

```
ft/
  cmd/
    root.go           -- cobra root command
    init.go           -- ft init
    sync.go           -- ft sync
    list.go           -- ft list
    show.go           -- ft show
    status.go         -- ft status
  internal/
    db/
      db.go           -- database connection, WAL mode setup
      migrate.go      -- migration system
      migrations/     -- migration SQL files or embedded strings
    parser/
      parser.go       -- gherkin .ft file parsing
    sync/
      sync.go         -- file/scenario reconciliation logic
    testlinks/
      testlinks.go    -- test link scanning
  main.go             -- entry point
  go.mod
  go.sum
```

- `cmd/` — cobra commands, thin wrappers that call into `internal/`
- `internal/db/` — database access, migrations
- `internal/parser/` — gherkin parsing, syntax error handling
- `internal/sync/` — reconciliation logic shared between CLI and eventual daemon
- `internal/testlinks/` — scanning non-.ft files for `@ft:` tags

## Build

```
go build -o ft .
```

## Test

```
go test ./...
```
