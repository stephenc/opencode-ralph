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
- [ ] Handle opencode exit codes properly (currently warns but continues)
- [ ] Add --model flag to specify AI model

## Notes System
- [x] Extract `<ralph_notes>...</ralph_notes>` from opencode output
- [x] Append extracted notes to .ralph/notes.md with timestamp
- [x] Handle missing or malformed notes gracefully

## Git Integration
- [x] Check if in git repository
- [x] Auto-commit after each iteration with message format
- [x] Handle commit failures gracefully (log and continue)
- [ ] Add --no-commit flag to disable auto-commit

## Orchestration Loop
- [x] Implement main loop with iteration counter
- [x] Respect max-iterations limit
- [x] Detect `<ralph_status>COMPLETE</ralph_status>` signal from output
- [ ] Add pause between iterations (optional delay)

## State Management
- [x] Create .ralph/ directory if not exists
- [x] Track iteration timestamps for rate limiting
- [x] Prune old timestamps (keep 24 hours)
- [ ] Implement lock file to prevent concurrent runs
- [ ] Clean up lock on exit (including signals)

## Polish
- [x] Add --help with usage examples
- [x] Add --dry-run to show prompt without executing
- [ ] Add colored output for status messages
- [ ] Handle Ctrl+C gracefully (signal handling)
- [ ] Add --quiet mode to suppress status output
