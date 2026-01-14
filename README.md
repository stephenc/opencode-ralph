# opencode-ralph

A small Go program that orchestrates iterative development loops using `opencode`.

`opencode-ralph` repeatedly constructs a composite prompt (prompt + conventions + specs + last notes), launches `opencode`, captures `<ralph_notes>...</ralph_notes>` and appends them to `.ralph/notes.md`, and stops when the agent outputs `<ralph_status>COMPLETE</ralph_status>`.

## Requirements

- Go 1.21+
- `opencode` available on your `PATH`

## Install / Build

```bash
go build -o opencode-ralph .
```

## Quick Start

```bash
./opencode-ralph init
./opencode-ralph run
```

This creates/editable files:

- `PROMPT.md` (agent instructions)
- `CONVENTIONS.md` (project conventions; build/test requirements)
- `SPECS.md` (the checklist that drives the loop)
- `.ralph/notes.md` (accumulated notes captured from `<ralph_notes>`)
- `.ralph/config.json` (optional config)

## Commands

- `init`: create `PROMPT.md`, `CONVENTIONS.md`, and `SPECS.md` from templates (only if missing)
- `manual`: run exactly one iteration
- `run`: run multiple iterations until complete (default)
- `config`: view/set/reset configuration

Run `./opencode-ralph help` to see all flags.

## Configuration

Configuration lives at `.ralph/config.json` and is overridden by CLI flags.

Keys:

- `prompt_file`
- `conventions_file`
- `specs_file`
- `max_iterations`
- `max_per_hour`
- `max_per_day`
- `model`

Example:

```bash
./opencode-ralph config set max_iterations 25
./opencode-ralph config set model ollama/qwen3-coder:30b
```

## OpenCode Passthrough Flags

`opencode-ralph` exposes a small subset of `opencode run` flags:

- `--model`
- `--agent`
- `--format` (`default` or `json`)
- `--continue` / `--session`
- `--file` (repeatable) / `--title`
- `--attach` / `--port`
- `--variant`

## Output / UX

- `--quiet` suppresses `opencode-ralph` status output (but still streams `opencode` output).
- A summary is printed at the end of a run (suppressed by `--quiet`).

## Notes

- `.ralph/state.json` is runtime state (rate limiting / timestamps).
- `.ralph/lock` prevents concurrent runs.
