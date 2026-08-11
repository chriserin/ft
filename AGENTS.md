# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

`ft` (module `github.com/chriserin/ft`) is a Go CLI for tracking feature scenarios in Gherkin-style `.ft` files, with metadata (status, test links) in a local SQLite database. It's explicitly "AI-first" — most usage is an AI agent reading and updating scenario status during a coding session, not a human typing commands.

**Run `ft agent-instructions` to print this project's rules for how an agent should use its own status workflow** .

## Commands

- Build: `go build -o ft .`
- Run all tests: `go test ./...`
- Run a single test: `go test ./cmd/ -run TestName -v` (swap the package path for `./internal/db/`, `./internal/parser/`, etc.)
- Vet (this project's lint step — no golangci-lint or other linter is configured): `go vet ./...`
- Cross-platform release build (all GOOS/GOARCH combos into `dist/`): `relscripts/build.sh [version]`

## Architecture

### Parser: two-layer model (`internal/parser`)

- Layer 1 (`ast.go`, `parser.go`) — a tree-sitter-compatible AST straight from Gherkin syntax: `Document` → `Feature` → `Background`/`Scenarios` → `StepGroups`.
- Layer 2 (`transform.go`) — `Transform()` converts the Layer 1 AST into `ParsedFile`/`ParsedScenario`, the flattened model the rest of the app actually consumes (extracts the `@ft:<id>` tag into `FtTag`, computes each scenario's `Content`, etc.). Changing parsing behavior almost always means touching both layers.

### Database (`internal/db`)

- `Store` wraps `*sql.DB` and is the only place raw SQL lives — `cmd/` never touches `*sql.DB` directly.
- Schema is an append-only list of raw SQL strings in `migrate.go`, applied in order and tracked via a `schema_version` table. Add new schema changes by appending a new entry; never edit a past one.
- `internal/db/dbtest` is a separate test-only package with direct DB access/assertions, kept apart from `Store` specifically so test-only queries never leak into the production data-access API.

### `fts/statuses.csv`: a second, git-tracked source of truth

`fts/ft.db` is local and gitignored; `fts/statuses.csv` is git-tracked and holds each scenario's _current_ status only — not a full history log, which stays DB-only (see `design/STATUSES_FILE.md`). `Store.InsertStatus` writes both together. `ft sync` reconciles the two on every run, but the two directions are **not** symmetric:

- DB → file runs unconditionally on every sync (cheap full rewrite from DB state — self-healing, and what backfills history for a project that adopted this file after already accumulating status).
- file → DB (replaying the file into the DB) only fires when the DB has _zero_ status rows at all — the specific, unambiguous signal that `fts/ft.db` was lost/deleted, not just "the file happens to have rows the DB doesn't."

### `cmd/` conventions

- Every command is a thin cobra wrapper around an exported `RunX(w io.Writer, ...) error` function; tests call `RunX` directly and never exercise the cobra plumbing.
- In `ft sync`, a scenario's `@ft:<id>` tag is its durable identity. A file not yet tracked in the DB will still adopt an existing tag if that id isn't already claimed by a different scenario (`insertOrAdoptScenario` / `ScenarioExists`) — required for rebuilding after `fts/ft.db` is deleted. If the id _is_ already claimed, the tag is stripped and a fresh one assigned, to protect against stale or copy-pasted tags.
- Test-link discovery (`ft sync` scanning `_test.go` files for `@ft:<id>`) uses `go/scanner` token-by-token, not regex or a full AST — it looks for the tag inside a comment token positioned immediately above a `func TestXxx` line.
- `ft agent-instructions`' output isn't a Go string literal — it's embedded from `cmd/agent_instructions.md` via `//go:embed`, so it stays editable as plain markdown (the content is backtick-heavy, which makes it painful to keep as a raw Go string).

### This repo tracks itself

`fts/*.ft` and `phases/PHASES.md` are this project's own feature/phase backlog, tracked with the tool being built. `design/*.md` holds the design rationale behind each phase, one doc per major feature (e.g. `STATUSES_FILE.md`, `AGENT_INSTRUCTIONS.md`). Expect design decisions and status-workflow rules to be spread across a design doc, the phase description, and the `.ft` scenario file for that phase — check all three before assuming current behavior.
