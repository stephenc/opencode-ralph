package ralph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func withTempCWD(t *testing.T) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}

func TestConfigRoundTrip(t *testing.T) {
	withTempCWD(t)

	cfg := DefaultConfig()
	cfg.PromptFile = "PROMPT.custom.md"
	cfg.MaxIterations = 123

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded := LoadConfig()
	if loaded.PromptFile != cfg.PromptFile {
		t.Fatalf("PromptFile: got %q want %q", loaded.PromptFile, cfg.PromptFile)
	}
	if loaded.MaxIterations != cfg.MaxIterations {
		t.Fatalf("MaxIterations: got %d want %d", loaded.MaxIterations, cfg.MaxIterations)
	}
}

func TestConfigSet(t *testing.T) {
	withTempCWD(t)

	if err := ConfigSet("prompt_file", "PROMPT2.md"); err != nil {
		t.Fatalf("ConfigSet prompt_file: %v", err)
	}
	cfg := LoadConfig()
	if cfg.PromptFile != "PROMPT2.md" {
		t.Fatalf("PromptFile: got %q want %q", cfg.PromptFile, "PROMPT2.md")
	}

	if err := ConfigSet("max_iterations", "5"); err != nil {
		t.Fatalf("ConfigSet max_iterations: %v", err)
	}
	cfg = LoadConfig()
	if cfg.MaxIterations != 5 {
		t.Fatalf("MaxIterations: got %d want %d", cfg.MaxIterations, 5)
	}

	if err := ConfigSet("unknown_key", "x"); err == nil {
		t.Fatalf("expected error for unknown_key")
	}
}

func TestConstructPromptIncludesSpecsAndNote(t *testing.T) {
	promptMD := "PROMPT BODY"
	conventionsMD := "CONVENTIONS BODY"
	specsMD := "- [ ] a task"
	notesMD := "notes"

	out := constructPrompt(promptMD, conventionsMD, specsMD, notesMD, 3, 50)

	if !strings.Contains(out, "NOTE: The full, current contents of the specs") {
		t.Fatalf("expected note about specs inclusion")
	}
	if !strings.Contains(out, "<specs>") || !strings.Contains(out, "</specs>") {
		t.Fatalf("expected <specs> tags")
	}
	if !strings.Contains(out, specsMD) {
		t.Fatalf("expected specs content")
	}
	if !strings.Contains(out, "Iteration: 3 of 50") {
		t.Fatalf("expected iteration line")
	}
}

func TestExtractNotes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "missing", in: "no notes", want: ""},
		{name: "present", in: "<ralph_notes>\nhello\n</ralph_notes>", want: "hello"},
		{name: "malformed", in: "<ralph_notes>oops", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNotes(tt.in)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestIsComplete(t *testing.T) {
	if isComplete("<ralph_status>COMPLETE</ralph_status>") != true {
		t.Fatalf("expected COMPLETE to be detected")
	}
	if isComplete("<ralph_status>INCOMPLETE</ralph_status>") != false {
		t.Fatalf("did not expect INCOMPLETE to be detected")
	}
}

func TestAppendNotesCreatesEntry(t *testing.T) {
	withTempCWD(t)

	if err := os.MkdirAll(ralphDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", ralphDir, err)
	}

	if err := appendNotes("some notes", 7); err != nil {
		t.Fatalf("appendNotes: %v", err)
	}

	data, err := os.ReadFile(notesFile)
	if err != nil {
		t.Fatalf("read notes file: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "## Iteration 7") {
		t.Fatalf("expected iteration header")
	}
	if !strings.Contains(text, "some notes") {
		t.Fatalf("expected note body")
	}
}

func TestAcquireLockStaleLockGetsCleaned(t *testing.T) {
	withTempCWD(t)

	lockPath := filepath.Join(ralphDir, "lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}

	// Create a lock file with a PID that should not exist.
	if err := os.WriteFile(lockPath, []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	locked, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquireLock: %v", err)
	}
	if !locked {
		t.Fatalf("expected lock to be acquired")
	}
	if err := releaseLock(lockPath); err != nil {
		t.Fatalf("releaseLock: %v", err)
	}
}

func TestAcquireLockFailsWhenHeld(t *testing.T) {
	withTempCWD(t)

	lockPath := filepath.Join(ralphDir, "lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}

	locked, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquireLock (first): %v", err)
	}
	if !locked {
		t.Fatalf("expected first lock to succeed")
	}
	t.Cleanup(func() {
		_ = releaseLock(lockPath)
	})

	locked2, err := acquireLock(lockPath)
	if err == nil {
		t.Fatalf("expected second acquireLock to fail")
	}
	if locked2 {
		t.Fatalf("expected locked=false when failing")
	}
}

func TestCountRecentIterations(t *testing.T) {
	now := time.Now().Unix()
	timestamps := []int64{
		now - int64(30*time.Minute.Seconds()),
		now - int64(2*time.Hour.Seconds()),
		now - int64(25*time.Hour.Seconds()),
	}

	hourCount, dayCount := countRecentIterations(timestamps)
	if hourCount != 1 {
		t.Fatalf("hourCount: got %d want %d", hourCount, 1)
	}
	if dayCount != 2 {
		t.Fatalf("dayCount: got %d want %d", dayCount, 2)
	}
}

func TestPruneOldTimestamps(t *testing.T) {
	now := time.Now().Unix()
	state := State{
		Timestamps: []int64{
			now - int64(23*time.Hour.Seconds()),
			now - int64(25*time.Hour.Seconds()),
		},
	}

	pruneOldTimestamps(&state)
	if len(state.Timestamps) != 1 {
		t.Fatalf("timestamps kept: got %d want %d", len(state.Timestamps), 1)
	}
}

func TestOrchestratorUsesRunnerAndStopsOnComplete(t *testing.T) {
	withTempCWD(t)

	cfg := DefaultConfig()
	cfg.PromptFile = "PROMPT.md"
	cfg.ConventionsFile = "CONVENTIONS.md"
	cfg.SpecsFile = "SPECS.md"

	if err := os.WriteFile(cfg.PromptFile, []byte("PROMPT"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	if err := os.WriteFile(cfg.ConventionsFile, []byte("CONVENTIONS"), 0o644); err != nil {
		t.Fatalf("write conventions: %v", err)
	}
	if err := os.WriteFile(cfg.SpecsFile, []byte("SPECS"), 0o644); err != nil {
		t.Fatalf("write specs: %v", err)
	}

	var calls int
	runner := &fakeRunner{
		runFunc: func(args OpencodeRunArgs) (string, error) {
			calls++
			if args.Prompt == "" {
				return "", fmt.Errorf("expected prompt to be set")
			}
			return "<ralph_status>COMPLETE</ralph_status>", nil
		},
	}

	if err := runIterationsWithRunner(cfg, 3, 0, 0, "", "", "", "", "", 0, false, "", nil, "", true, false, false, 0, runner); err != nil {
		t.Fatalf("runIterationsWithRunner: %v", err)
	}
	if calls != 1 {
		t.Fatalf("runner calls: got %d want %d", calls, 1)
	}
}

type fakeRunner struct {
	runFunc func(OpencodeRunArgs) (string, error)
}

func (r *fakeRunner) Run(args OpencodeRunArgs) (string, error) {
	if r.runFunc == nil {
		return "", fmt.Errorf("fakeRunner missing runFunc")
	}
	return r.runFunc(args)
}
