# Project Conventions

## Language & Tools
- **Language**: Go 1.21+
- **Formatter**: `gofmt` (run before committing)
- **Linter**: `golangci-lint` (if available)

## Code Style

### Error Handling
- Always check errors; never ignore with `_`
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Return early on errors (no deep nesting)

### Naming
- Use `camelCase` for unexported, `PascalCase` for exported
- Acronyms: `URL`, `HTTP`, `ID` (all caps)
- Descriptive names over comments

### File Organization
```
opencode-ralph/
├── main.go           # Entry point, CLI parsing
├── orchestrator.go   # Main loop logic
├── specs.go          # SPECS.md parsing
├── prompt.go         # Prompt construction
├── notes.go          # Notes extraction/persistence
├── git.go            # Git operations
├── PROMPT.md         # Instructions for opencode
├── CONVENTIONS.md    # This file
├── SPECS.md          # Task checklist
└── .ralph/
    └── notes.md      # Accumulated notes
```

### Functions
- Keep functions short (<50 lines preferred)
- Single responsibility
- Document exported functions

### Testing
- Test files: `*_test.go`
- Table-driven tests preferred
- Test edge cases and error paths

## Git Commits
- Format: `ralph[N]: brief description`
- Example: `ralph[3]: implement specs parser`
- Auto-commits are made by the orchestrator

## Dependencies
- Minimize external dependencies
- Standard library preferred
- If needed: `cobra` for CLI, but flags package is fine
