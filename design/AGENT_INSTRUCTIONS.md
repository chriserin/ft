# `ft` — `ft agent-instructions`

## Purpose

`ft` is explicitly "AI-first feature tracking" (see README.md) — most `ft`
usage is an AI agent reading and updating scenario state during a coding
session, not a human typing commands. But nothing today tells an agent
*how* to use it: which statuses it's allowed to set, which are system-owned,
where the agent's own authority ends and a human's begins in the
implement → test → accept loop. That knowledge currently only exists in
design docs a human reads, or gets re-derived by each agent from README.md
and trial and error.

`ft agent-instructions` prints a ready-to-use block of instructions — written for
an AI to follow, not a human to browse — covering both the CLI (what each
command does, when to run it) and the status vocabulary (what each status
means, which ones the AI must never set by hand, and critically, that the
agent's own authority stops at `fulfilled` — `accepted` is a human's call,
never the agent's). The intent is for a human or tooling to pipe this into
wherever their agent's instructions live:

```bash
ft agent-instructions >> CLAUDE.md
ft agent-instructions >> AGENTS.md
```

or for an agent harness to invoke it directly (a Claude Code skill, an MCP
tool, a shell hook) rather than requiring a human to remember to wire it up.

---

## Command

```
ft agent-instructions
```

No required arguments. Always prints to stdout and exits 0. Does not require
`ft init` to have been run first — it's useful even as the very first
thing an agent runs in an uninitialized project ("how do I set this up"),
so it must not fail just because `fts/` doesn't exist yet.

---

## Output Content

The printed text is organized into the sections below. This isn't
documentation *about* `ft` for a human to read casually — every line should
be something an agent can act on directly.

### 1. Core concepts

- What a **scenario** is, and that `@ft:<id>` is its permanent identity —
  never hand-edit or renumber a tag; `ft sync` owns that.
- That a **status** is free-text metadata on a scenario, and a scenario with
  none set reads as `no-activity`.
- That `fts/ft.db` is local and gitignored, `fts/statuses.csv` is a
  git-tracked snapshot of current status (see
  [STATUSES_FILE.md](STATUSES_FILE.md)) — the agent should never hand-edit
  either file; both are `ft`-managed.

### 2. Command reference

One line per command, phrased as an instruction rather than a description —
"run `ft sync` after editing `.ft` files or test files" rather than "`ft
sync` scans for changes." Pulled from the same command set as README.md's
table (`init`, `sync`, `list`, `show`, `status`, `tests`), but written for an
agent deciding what to run next, not a human skimming a table.

### 3. Status vocabulary

This is the part actually motivating this command — see
[STATUSES.md](STATUSES.md) for the authoritative definitions; this section
restates them as behavioral rules:

Three tiers, by who's allowed to set them — this is the single most
important thing the printed text needs to get across, since nothing in the
tool itself enforces it:

- **System-set — never set these via `ft status`, agent or human:**
  - `modified` — set automatically by `ft sync` when a scenario's step
    content changes after it was previously touched.
  - `removed` — set automatically by `ft sync` when a scenario disappears
    from its file but has prior status history.
  - `restored` — set automatically by `ft sync` when a removed scenario's
    tag reappears in a file.
- **Human-set — the agent must never set any of these:**
  - `ready` — a human has reviewed the scenario's spec and judged it
    finalized and ready to implement. The agent doesn't decide what's
    ready to build; it only picks up work a human already marked that way.
  - `accepted` — a human has confirmed `fulfilled` work is actually
    correct. Even when the agent is fully confident in its own testing,
    that confidence is exactly what `fulfilled` communicates — `accepted`
    is a separate, human judgment call.
  - `rejected` — a human reviewed `fulfilled` work and found it wrong.
- **Agent-set — the implementation loop:**
  - `in-progress` — implementation has started on a `ready` scenario.
  - `fulfilled` — the agent believes implementation is complete: a test
    exists, is linked (`@ft:<id>` above the test function), and passes.
    This is as far as the agent's own authority goes; it never continues on
    to `accepted` itself.

The instruction for `rejected` is stricter than "fix it and re-fulfill":
**never implement a scenario unless its status is `ready`.** `rejected`
doesn't mean "go fix this" — it means the agent's job on this scenario is
over until a human re-opens it. The agent must not touch the
implementation, and must not set `fulfilled` again on its own initiative.
Recovery back to `ready` happens one of two ways, both requiring a human:
- a human reviews the rejection and sets the status straight back to
  `ready` (nothing about the spec needed to change — it was an
  implementation bug), or
- a human edits the scenario's spec in its `.ft` file to clarify or correct
  it; `ft sync` then sets `modified` automatically (system-set, not
  agent-set), and a human still has to promote that to `ready` before the
  agent may pick it up again.

Either way, the agent waits for `ready`. It never infers permission to
resume from seeing `rejected` or `modified` on its own. (This project's own
history has an example of the underlying problem, if not this exact
recovery path — see the git log around `fts/statuses.csv` reconciliation,
where `ft:214`/`ft:215` were rejected, then fixed and re-verified before
being marked `accepted` by a human.)

None of this is enforced by the tool (`ft status` accepts any text — see
`@ft:59` in `phase_6_ft_status.ft`) — nothing stops an agent from running
`ft status <id> ready` or `ft status <id> accepted` directly, so these
boundaries have to be stated as explicit instructions here, not relied on
as a system guarantee.

### 4. Workflow

A short numbered loop tying the above together: only start on scenarios a
human has already marked `ready` — never `rejected`, never `modified`,
never `no-activity`, never anything else; mark `in-progress`; implement
with a linked test; verify the test passes; mark `fulfilled` —
**stop there**. If a scenario comes back `rejected`, that's the end of the
agent's involvement until a human moves it back to `ready` (directly, or by
editing the spec, which `ft sync` turns into `modified` first, still
requiring a human to promote it to `ready`). The instruction to never
implement anything that isn't currently `ready`, and to never set `ready`
or `accepted` itself, is the part meant to actually change agent behavior
session to session, not just inform it.

---

## Implementation Notes

- New file `cmd/agent_instructions.go`, registering `agent-instructions` as a cobra
  subcommand (`Use: "agent-instructions"`), consistent with the rest of the
  command set.
- The output is static content, held in `cmd/agent_instructions.md` and
  pulled in via `//go:embed` — a plain markdown file rather than a Go
  string literal, since the content is backtick-heavy (code spans) and a
  raw Go string can't contain a backtick without concatenation tricks.
  No templating against the live DB, no per-project override file.
  Nothing about scenario counts or current project state belongs in this
  output; it's instructions for *how to behave*, not a status report
  (`ft status` already exists for that). No dependency on
  `db.DataDirExists()` or `OpenProjectStore()` — the command never touches
  the DB or `fts/` at all.
- Format is markdown, printed straight to stdout — the intended usage is
  `ft agent-instructions >> CLAUDE.md` (or `AGENTS.md`, etc.), and once it's
  there, a human can hand-edit that copy same as anything else in those
  files. No `--format` flag, no per-tool variants baked into the command
  itself.

## Considered and Rejected: Project Override File

An earlier version of this design let a project override the built-in
default entirely via `fts/agent-instructions.md` — if present, print its content
verbatim instead. Cut for now:

- **No real second use case.** This project is currently the only consumer
  of `ft agent-instructions`, and it uses the default. Building customization
  before a project actually needs different conventions is speculative.
- **The `>> CLAUDE.md` usage already covers most of the need.** Once printed
  into `CLAUDE.md`, a human already has an editable copy — they don't need
  `ft` itself to know about overrides for that. An override file only earns
  its keep if something re-invokes `ft agent-instructions` live every session
  instead of reading a cached file, which nothing does today.
- **"Verbatim, no merge" was a liability, not just simplicity.** A project
  that overrode would have had to copy and maintain the *entire* prompt
  itself, including the core-concepts and command-reference sections —
  facts about the tool, not conventions. Any future change to `ft` would
  silently go stale in every project's override.

Cheap to revisit if a real second project shows up wanting different
conventions — this cut doesn't foreclose adding it back later.
