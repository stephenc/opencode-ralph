# opencode-ralph Specifications

Implementation checklist for the Go-based Ralph Wiggum orchestrator that drives `opencode`.

## Core Infrastructure
- [x] Initialize Go module (`go mod init`)
- [x] Create initial CLI with subcommands (init, manual, run, config, help)
- [x] Implement Config struct with file paths and rate limits
- [x] Use go:embed for templates

## File Operations
- [x] Implement file loader for configurable prompt, conventions, specs files
- [x] Implement notes loader/creator for `.ralph/notes.md`
- [x] Create templates directory with embedded `PROMPT.md`, `CONVENTIONS.md`, `SPECS.md`

## Configuration System
- [x] Create `.ralph/config.json` support
- [x] Add config subcommand (view, set, reset)
- [x] CLI options override config values
- [x] Configurable file names (`prompt_file`, `conventions_file`, `specs_file`)
- [x] Configurable rate limits (`max_iterations`, `max_per_hour`, `max_per_day`)

## Prompt Construction
- [x] Build composite prompt from prompt + conventions + specs + notes
- [x] Include iteration number and max in prompt
- [x] Format with XML-style tags for sections
- [x] Use `<ralph_notes>` and `<ralph_status>` tag names

## OpenCode Integration
- [x] Launch `opencode` subprocess with constructed prompt
- [x] Stream stdout/stderr in real-time (verbose mode)
- [x] Capture full output for notes extraction
- [x] Handle `opencode` exit codes properly (warn but continue)
- [x] Add `--model` flag to specify AI model

## Notes System
- [x] Extract `<ralph_notes>...</ralph_notes>` from opencode output
- [x] Append extracted notes to `.ralph/notes.md` with timestamp
- [x] Handle missing or malformed notes gracefully
- [x] Only include last iteration's notes in prompt (not full history)

## Git Integration
**Design Note**: Ralph does NOT manage git commits directly. `opencode` (and its agent) performs commits.

- [ ] Remove ralph's auto-commit code (autoCommit function and any calls)
- [x] Remove `--no-commit` flag (no longer needed)
- [x] Update `PROMPT.md` to instruct the agent to commit after completing each task
- [x] Require conventional commits style (`fix:`, `feat:`, `docs:`, `test:`, `chore:`, etc.)

## Orchestration Loop
- [x] Implement main loop with iteration counter
- [x] Respect max-iterations limit
- [x] Detect `<ralph_status>COMPLETE</ralph_status>` signal from output
- [x] Re-read all files before each iteration (allows live editing of specs/prompt/conventions)
- [x] Add 2 second default pause between iterations (configurable via `--delay`)

## State Management
- [x] Create `.ralph/` directory if not exists
- [x] Track iteration timestamps for rate limiting
- [x] Prune old timestamps (keep 24 hours)
- [x] Implement lock file to prevent concurrent runs
- [x] Improve lock handling: detect stale lock via PID and clean it up
- [x] Clean up lock on exit (including SIGINT/SIGTERM)

## CLI + Package Structure Refactor
Goal: match a clean Go project layout and keep specs implementation-agnostic.

- [ ] Migrate CLI to `cobra` (root command + `init`, `run`, `manual`, `config` subcommands)
- [ ] Restructure packages to:
  - `main.go` (calls `cmd.Execute()`)
  - `cmd/` (cobra commands and flags)
  - `internal/ralph/` (all orchestration logic)
- [ ] Split `internal/ralph/` into focused modules:
  - `config.go` (config + persistence)
  - `state.go` (state + rate limiting)
  - `orchestrator.go` (iteration loop)
  - `runner.go` (opencode runner interface + implementation)
  - `prompt.go` (prompt construction)
  - `notes.go` (notes extraction/persistence)
  - `lock.go` (lock acquire/release)
  - `signal.go` (signal handling)
  - `color.go` (banner + colored status output)
  - `templates.go` (+ embedded templates)
- [ ] Move embedded templates into `internal/ralph/templates/` and update init to read from the embedded template FS
- [ ] Keep current CLI behavior/UX equivalent after refactor (same defaults and core flags)

## opencode Runner Passthrough Flags
Expose a small, explicit set of `opencode run` flags via opencode-ralph (do not attempt to mirror every flag).

- [x] Add `--agent` passthrough to `opencode run --agent`
- [x] Add `--format` passthrough to `opencode run --format` (`default` or `json`)
- [x] Add session passthroughs:
  - `--continue` to `opencode run --continue`
  - `--session` to `opencode run --session`
- [x] Add message attachment passthroughs:
  - `--file` to `opencode run --file` (repeatable)
  - `--title` to `opencode run --title`
- [x] Add remote attach passthroughs:
  - `--attach` to `opencode run --attach`
  - `--port` to `opencode run --port`
- [x] Add `--variant` passthrough to `opencode run --variant`

## Output / UX Polish
- [x] Add `--quiet` flag that hides opencode-ralph banner/status output
- [x] In `--quiet` mode, always stream `opencode` output (even if not `--verbose`)
- [ ] Add colored output for status messages and iteration headers
- [ ] Respect `NO_COLOR` environment variable for all colors
- [ ] Display ASCII art banner on startup (disabled by `--quiet`)
- [ ] Display summary at end of run (iterations, duration, final status; suppressed by `--quiet`)

## Robustness
- [x] Handle Ctrl+C gracefully (SIGINT/SIGTERM): release lock and exit cleanly

## Testing
- [ ] Add unit tests for `internal/ralph` modules (config/state/prompt/notes/lock)
- [ ] Add an orchestrator integration test using a mock runner (no real `opencode` invocation)
- [ ] Update `CONVENTIONS.md` to include `go test ./...` and a coverage target for `internal/ralph` (>= 80%)

## Documentation
- [ ] Create `README.md` for the repository (usage, examples, configuration)
