# Project Conventions

## Language & Tools
- **Language**: Go 1.21+
- **Formatter**: `gofmt` (run before committing)
- **Linter**: `golangci-lint` (if available)

## Build & Verify
Before marking any task complete, you MUST verify your changes:
```bash
gofmt -w .
go test ./...
go build -o opencode-ralph .
```
If the build or tests fail, fix all errors before proceeding. Do not mark a task complete if the code does not compile.

## Coverage Target
After the `internal/ralph` refactor lands, aim for >= 80% coverage for `internal/ralph`.

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
Use conventional commits format:
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `refactor:` code change that neither fixes a bug nor adds a feature
- `chore:` maintenance tasks
- `test:` adding or updating tests

Examples:
- `feat: add ASCII art banner on startup`
- `fix: handle missing config file gracefully`
- `refactor: remove unused autoCommit function`

## Dependencies
- Minimize external dependencies
- Standard library preferred
- If needed: `cobra` for CLI, but flags package is fine
