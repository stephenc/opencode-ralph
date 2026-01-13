# opencode-ralph Specifications

Implementation checklist for the Go-based Ralph Wiggum orchestrator.

## Core Infrastructure
- [x] Initialize Go module (`go mod init`)
- [x] Create main.go with CLI subcommands (init, manual, run, config, help)
- [x] Implement Config struct with file paths and rate limits
- [x] Use go:embed for templates

## File Operations
- [x] Implement file loader for configurable prompt, conventions, specs files
- [x] Implement notes loader/creator for .ralph/notes.md
- [x] Create templates/ directory with embedded PROMPT.md, CONVENTIONS.md, SPECS.md

## Configuration System
- [x] Create .ralph/config.json support
- [x] Add config subcommand (view, set, reset)
- [x] CLI options override config values
- [x] Configurable file names (prompt_file, conventions_file, specs_file)
- [x] Configurable rate limits (max_iterations, max_per_hour, max_per_day)

## Prompt Construction
- [x] Build composite prompt from prompt + conventions + specs + notes
- [x] Include iteration number and max in prompt
- [x] Format with XML-style tags for sections
- [x] Use <ralph_notes> and <ralph_status> tag names

## OpenCode Integration
- [x] Launch opencode subprocess with constructed prompt
- [x] Stream stdout/stderr in real-time (verbose mode)
- [x] Capture full output for notes extraction
- [x] Handle opencode exit codes properly (currently warns but continues)
- [x] Add --model flag to specify AI model

## Notes System
- [x] Extract `<ralph_notes>...</ralph_notes>` from opencode output
- [x] Append extracted notes to .ralph/notes.md with timestamp (full history kept on disk)
- [x] Handle missing or malformed notes gracefully
- [ ] Only include LAST iteration's notes in prompt (not full history) to reduce context usage

## Git Integration
**Design Note**: Ralph does NOT manage git commits directly. Instead, opencode handles
commits via its own auto-commit mechanism. Ralph orchestrates iterations and lets
opencode commit as part of each run.

- [ ] Remove ralph's auto-commit code (autoCommit function and calls)
- [ ] Remove --no-commit flag (no longer needed)
- [ ] Update PROMPT.md to instruct opencode to commit after completing each task
- [ ] Require conventional commits style (fix:, feat:, docs:, test:, chore:, etc.)

## Orchestration Loop
- [x] Implement main loop with iteration counter
- [x] Respect max-iterations limit
- [x] Detect `<ralph_status>COMPLETE</ralph_status>` signal from output
- [x] Re-read all files before each iteration (allows live editing of specs/prompt/conventions)
- [ ] Add 2 second default pause between iterations (configurable via --delay flag)

## State Management
- [x] Create .ralph/ directory if not exists
- [x] Track iteration timestamps for rate limiting
- [x] Prune old timestamps (keep 24 hours)
- [ ] Implement lock file to prevent concurrent runs
- [ ] Clean up lock on exit (including signals)

## Polish
- [x] Add --help with usage examples
- [x] Add --dry-run to show prompt without executing
- [ ] Display ASCII art banner on startup (by default, disable with --quiet)
- [ ] Add colored output for status messages
- [ ] Handle Ctrl+C gracefully (signal handling)
- [ ] Add --quiet mode to suppress status output and banner

## Documentation
- [ ] Create README.md for the repository (usage, examples, configuration)
