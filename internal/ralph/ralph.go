package ralph

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed templates/*
var templates embed.FS

// Config holds project configuration.
type Config struct {
	PromptFile      string `json:"prompt_file"`
	ConventionsFile string `json:"conventions_file"`
	SpecsFile       string `json:"specs_file"`
	MaxIterations   int    `json:"max_iterations"`
	MaxPerHour      int    `json:"max_per_hour"`
	MaxPerDay       int    `json:"max_per_day"`
	Model           string `json:"model,omitempty"`
}

// State tracks iteration history for rate limiting.
type State struct {
	TotalIterations int       `json:"total_iterations"`
	Timestamps      []int64   `json:"timestamps"`
	LastRun         time.Time `json:"last_run"`
}

// RunOptions are CLI overrides for a run.
type RunOptions struct {
	MaxIterations   int
	MaxPerHour      int
	MaxPerDay       int
	Prompt          string
	Conventions     string
	Specs           string
	Agent           string
	Format          string
	ContinueSession bool
	Session         string
	Files           []string
	Title           string
	Variant         string
	Attach          string
	Port            int
	Quiet           bool
	Model           string
	Verbose         bool
	DryRun          bool
	Delay           float64
}

const (
	ralphDir   = ".ralph"
	configFile = ".ralph/config.json"
	stateFile  = ".ralph/state.json"
	notesFile  = ".ralph/notes.md"
	lockFile   = ".ralph/lock"
)

const banner = `
   ____  ____  ____  _   _
  / __ \/ __ \/ __ \/ | / /
 / / / / /_/ / /_/ /  |/ / 
/ /_/ / ____/ ____/ /|  /  
\____/_/   /_/   /_/ |_/   

opencode-ralph
`

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

func shouldUseColor(quiet bool) bool {
	if quiet {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func style(text string, codes ...string) string {
	if len(codes) == 0 {
		return text
	}

	var b strings.Builder
	for _, code := range codes {
		b.WriteString(code)
	}
	b.WriteString(text)
	b.WriteString(ansiReset)
	return b.String()
}

func styleIf(enabled bool, text string, codes ...string) string {
	if !enabled {
		return text
	}
	return style(text, codes...)
}

func statusStyle(status string) (string, []string) {
	switch strings.ToLower(status) {
	case "complete":
		return strings.ToUpper(status), []string{ansiGreen, ansiBold}
	case "rate_limited", "max_iterations":
		return strings.ToUpper(status), []string{ansiYellow, ansiBold}
	case "dry_run":
		return strings.ToUpper(status), []string{ansiCyan, ansiBold}
	case "unknown":
		return strings.ToUpper(status), []string{ansiGray}
	default:
		return strings.ToUpper(status), []string{ansiGray}
	}
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		PromptFile:      "PROMPT.md",
		ConventionsFile: "CONVENTIONS.md",
		SpecsFile:       "SPECS.md",
		MaxIterations:   50,
		MaxPerHour:      0,
		MaxPerDay:       0,
	}
}

// LoadConfig loads .ralph/config.json if present.
func LoadConfig() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// SaveConfig persists cfg to .ralph/config.json.
func SaveConfig(cfg Config) error {
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", ralphDir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configFile, err)
	}
	return nil
}

// ConfigView renders the current config as JSON.
func ConfigView() (string, error) {
	cfg := LoadConfig()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling config: %w", err)
	}
	return string(data), nil
}

// ConfigReset resets config to defaults.
func ConfigReset() error {
	cfg := DefaultConfig()
	return SaveConfig(cfg)
}

// ConfigSet updates a single config key.
func ConfigSet(key, value string) error {
	cfg := LoadConfig()

	switch key {
	case "prompt_file":
		cfg.PromptFile = value
	case "conventions_file":
		cfg.ConventionsFile = value
	case "specs_file":
		cfg.SpecsFile = value
	case "max_iterations":
		v, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("parsing max_iterations: %w", err)
		}
		cfg.MaxIterations = v
	case "max_per_hour":
		v, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("parsing max_per_hour: %w", err)
		}
		cfg.MaxPerHour = v
	case "max_per_day":
		v, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("parsing max_per_day: %w", err)
		}
		cfg.MaxPerDay = v
	case "model":
		cfg.Model = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return SaveConfig(cfg)
}

func parseInt(value string) (int, error) {
	var v int
	if _, err := fmt.Sscanf(value, "%d", &v); err != nil {
		return 0, err
	}
	return v, nil
}

// Init creates .ralph/ and initial files from templates.
func Init() error {
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", ralphDir, err)
	}

	cfg := LoadConfig()

	if err := createFromTemplate(cfg.PromptFile, "templates/PROMPT.md"); err != nil {
		return err
	}
	if err := createFromTemplate(cfg.ConventionsFile, "templates/CONVENTIONS.md"); err != nil {
		return err
	}
	if err := createFromTemplate(cfg.SpecsFile, "templates/SPECS.md"); err != nil {
		return err
	}

	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		if err := SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Created .ralph/config.json")
	}

	fmt.Printf("\nInitialization complete. Edit %s to define your tasks.\n", cfg.SpecsFile)
	return nil
}

func createFromTemplate(destPath, templatePath string) error {
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("%s already exists, skipping\n", destPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", destPath, err)
	}

	content, err := templates.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", templatePath, err)
	}

	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}

	fmt.Printf("Created %s\n", destPath)
	return nil
}

// RunWithOptions executes iterations using opts, falling back to defaults.
func RunWithOptions(opts RunOptions, defaultMaxIterations, defaultMaxPerHour, defaultMaxPerDay int) error {
	cfg := LoadConfig()

	maxIterations := opts.MaxIterations
	if maxIterations == 0 {
		maxIterations = defaultMaxIterations
	}

	maxPerHour := opts.MaxPerHour
	if maxPerHour == 0 {
		maxPerHour = defaultMaxPerHour
	}

	maxPerDay := opts.MaxPerDay
	if maxPerDay == 0 {
		maxPerDay = defaultMaxPerDay
	}

	if opts.Prompt != "" {
		cfg.PromptFile = opts.Prompt
	}
	if opts.Conventions != "" {
		cfg.ConventionsFile = opts.Conventions
	}
	if opts.Specs != "" {
		cfg.SpecsFile = opts.Specs
	}

	modelToUse := opts.Model
	if modelToUse == "" {
		modelToUse = cfg.Model
	}

	if opts.Format != "" && opts.Format != "default" && opts.Format != "json" {
		return fmt.Errorf("invalid --format value: %s (expected default or json)", opts.Format)
	}
	if opts.ContinueSession && opts.Session != "" {
		return fmt.Errorf("invalid flags: --continue and --session are mutually exclusive")
	}

	quiet := opts.Quiet
	if opts.DryRun {
		quiet = false
	}

	verbose := opts.Verbose || quiet
	if opts.DryRun {
		verbose = false
	}

	return runIterations(cfg, maxIterations, maxPerHour, maxPerDay, modelToUse, opts.Agent, opts.Format, opts.Variant, opts.Attach, opts.Port, opts.ContinueSession, opts.Session, opts.Files, opts.Title, quiet, verbose, opts.DryRun, opts.Delay)
}

type OpencodeRunArgs struct {
	Prompt          string
	Model           string
	Agent           string
	Format          string
	Variant         string
	Attach          string
	Port            int
	ContinueSession bool
	Session         string
	Files           []string
	Title           string
	Quiet           bool
	Verbose         bool
}

type OpencodeRunner interface {
	Run(args OpencodeRunArgs) (string, error)
}

type execOpencodeRunner struct{}

func (execOpencodeRunner) Run(args OpencodeRunArgs) (string, error) {
	return runOpencode(args)
}

func runIterations(cfg Config, maxIterations, maxPerHour, maxPerDay int, model string, agent string, format string, variant string, attach string, port int, continueSession bool, session string, files []string, title string, quiet bool, verbose, dryRun bool, delay float64) (err error) {
	return runIterationsWithRunner(cfg, maxIterations, maxPerHour, maxPerDay, model, agent, format, variant, attach, port, continueSession, session, files, title, quiet, verbose, dryRun, delay, execOpencodeRunner{})
}

func runIterationsWithRunner(cfg Config, maxIterations, maxPerHour, maxPerDay int, model string, agent string, format string, variant string, attach string, port int, continueSession bool, session string, files []string, title string, quiet bool, verbose, dryRun bool, delay float64, runner OpencodeRunner) (err error) {
	startTime := time.Now()
	showSummary := !quiet && !dryRun
	useColor := shouldUseColor(quiet)
	finalStatus := "unknown"
	sessionIterations := 0
	defer func() {
		if err != nil || !showSummary {
			return
		}
		duration := time.Since(startTime).Truncate(time.Millisecond)
		fmt.Println("\n--- Summary ---")
		fmt.Printf("Iterations: %d\n", sessionIterations)
		fmt.Printf("Duration: %s\n", duration)
		label, codes := statusStyle(finalStatus)
		fmt.Printf("Status: %s\n", styleIf(useColor, label, codes...))
	}()

	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", ralphDir, err)
	}

	locked, err := acquireLock(lockFile)
	if err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	if locked {
		stopSignalHandler := installLockSignalHandler(lockFile)
		defer stopSignalHandler()

		defer func() {
			if err := releaseLock(lockFile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to release lock: %v\n", err)
			}
		}()
	}

	state := loadState()

	if !quiet {
		fmt.Print(banner)
	}

	for i := 0; i < maxIterations; i++ {
		sessionIterations++
		state.TotalIterations++
		iteration := state.TotalIterations

		if !quiet {
			header := fmt.Sprintf("=== Iteration %d (session: %d/%d) ===", iteration, i+1, maxIterations)
			fmt.Printf("\n%s\n", styleIf(useColor, header, ansiCyan, ansiBold))
		}

		if maxPerHour > 0 || maxPerDay > 0 {
			hourCount, dayCount := countRecentIterations(state.Timestamps)
			if maxPerHour > 0 && hourCount >= maxPerHour {
				if !quiet {
					fmt.Printf("%s\n", styleIf(useColor, fmt.Sprintf("Rate limit reached: %d iterations in the past hour (max: %d)", hourCount, maxPerHour), ansiYellow, ansiBold))
				}
				finalStatus = "rate_limited"
				saveState(state)
				return nil
			}
			if maxPerDay > 0 && dayCount >= maxPerDay {
				if !quiet {
					fmt.Printf("%s\n", styleIf(useColor, fmt.Sprintf("Rate limit reached: %d iterations in the past day (max: %d)", dayCount, maxPerDay), ansiYellow, ansiBold))
				}
				finalStatus = "rate_limited"
				saveState(state)
				return nil
			}
			if !quiet {
				fmt.Printf("Rate: %d/hour, %d/day\n", hourCount, dayCount)
			}
		}

		promptMD, err := readFile(cfg.PromptFile)
		if err != nil {
			return fmt.Errorf("reading %s: %w", cfg.PromptFile, err)
		}
		conventionsMD, err := readFile(cfg.ConventionsFile)
		if err != nil {
			return fmt.Errorf("reading %s: %w", cfg.ConventionsFile, err)
		}
		specsMD, err := readFile(cfg.SpecsFile)
		if err != nil {
			return fmt.Errorf("reading %s: %w", cfg.SpecsFile, err)
		}
		notesMD := readFileOrDefault(notesFile, "No notes yet.")

		prompt := constructPrompt(promptMD, conventionsMD, specsMD, notesMD, iteration, maxIterations)
		if dryRun {
			fmt.Println("\n--- DRY RUN: Constructed Prompt ---")
			fmt.Println(prompt)
			fmt.Println("--- END DRY RUN ---")
			finalStatus = "dry_run"
			return nil
		}

		output, runErr := runner.Run(OpencodeRunArgs{
			Prompt:          prompt,
			Model:           model,
			Agent:           agent,
			Format:          format,
			Variant:         variant,
			Attach:          attach,
			Port:            port,
			ContinueSession: continueSession,
			Session:         session,
			Files:           files,
			Title:           title,
			Quiet:           quiet,
			Verbose:         verbose,
		})
		if runErr != nil {
			if !quiet {
				fmt.Printf("%s\n", styleIf(useColor, fmt.Sprintf("Warning: opencode exited with error: %v", runErr), ansiYellow, ansiBold))
			}
		}

		if notes := extractNotes(output); notes != "" {
			if err := appendNotes(notes, iteration); err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "Warning: failed to save notes: %v\n", err)
				}
			}
		}

		if isComplete(output) {
			finalStatus = "complete"
			if !quiet {
				fmt.Println(styleIf(useColor, "Received COMPLETE signal from opencode!", ansiGreen, ansiBold))
			}
			return nil
		}

		state.Timestamps = append(state.Timestamps, time.Now().Unix())
		state.LastRun = time.Now()
		pruneOldTimestamps(&state)
		saveState(state)

		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	if !quiet {
		fmt.Printf("%s\n", styleIf(useColor, fmt.Sprintf("Reached maximum iterations (%d)", maxIterations), ansiYellow, ansiBold))
	}
	finalStatus = "max_iterations"
	return nil
}

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{Timestamps: []int64{}}
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{Timestamps: []int64{}}
	}
	if state.Timestamps == nil {
		state.Timestamps = []int64{}
	}
	return state
}

func saveState(state State) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(stateFile, data, 0644)
}

func pruneOldTimestamps(state *State) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	var kept []int64
	for _, ts := range state.Timestamps {
		if ts > cutoff {
			kept = append(kept, ts)
		}
	}
	state.Timestamps = kept
}

func countRecentIterations(timestamps []int64) (hourCount, dayCount int) {
	now := time.Now()
	hourAgo := now.Add(-time.Hour).Unix()
	dayAgo := now.Add(-24 * time.Hour).Unix()

	for _, ts := range timestamps {
		if ts > dayAgo {
			dayCount++
			if ts > hourAgo {
				hourCount++
			}
		}
	}
	return
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readFileOrDefault(path, defaultValue string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultValue
	}
	return string(data)
}

func constructPrompt(promptMD, conventionsMD, specsMD, notesMD string, iteration, maxIterations int) string {
	return fmt.Sprintf(`You are operating in Ralph Wiggum mode.

## Context Files

<prompt>
%s
</prompt>

<conventions>
%s
</conventions>

NOTE: The full, current contents of the specs are included below in <specs>.
Do not re-read SPECS.md unless you have modified it and need to confirm your updates.

<specs>
%s
</specs>

<ralph_notes_history>
%s
</ralph_notes_history>

## Current Iteration
Iteration: %d of %d
`, promptMD, conventionsMD, specsMD, notesMD, iteration, maxIterations)
}

func runOpencode(runArgs OpencodeRunArgs) (string, error) {
	args := []string{"run"}
	if runArgs.Model != "" {
		args = append(args, "-m", runArgs.Model)
	}
	if runArgs.Agent != "" {
		args = append(args, "--agent", runArgs.Agent)
	}
	if runArgs.Format != "" {
		args = append(args, "--format", runArgs.Format)
	}
	if runArgs.Variant != "" {
		args = append(args, "--variant", runArgs.Variant)
	}
	if runArgs.Attach != "" {
		args = append(args, "--attach", runArgs.Attach)
	}
	if runArgs.Port != 0 {
		args = append(args, "--port", fmt.Sprintf("%d", runArgs.Port))
	}
	if runArgs.ContinueSession {
		args = append(args, "--continue")
	}
	if runArgs.Session != "" {
		args = append(args, "--session", runArgs.Session)
	}
	for _, file := range runArgs.Files {
		if file != "" {
			args = append(args, "--file", file)
		}
	}
	if runArgs.Title != "" {
		args = append(args, "--title", runArgs.Title)
	}
	args = append(args, runArgs.Prompt)
	cmd := exec.Command("opencode", args...)

	var output bytes.Buffer

	if runArgs.Verbose || runArgs.Quiet {
		cmd.Stdout = io.MultiWriter(os.Stdout, &output)
		cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	} else {
		cmd.Stdout = &output
		cmd.Stderr = &output
	}

	err := cmd.Run()
	if err != nil {
		return output.String(), err
	}
	return output.String(), nil
}

func extractNotes(output string) string {
	re := regexp.MustCompile(`(?s)<ralph_notes>(.*?)</ralph_notes>`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func isComplete(output string) bool {
	re := regexp.MustCompile(`(?si)<ralph_status>\s*COMPLETE\s*</ralph_status>`)
	return re.MatchString(output)
}

func appendNotes(notes string, iteration int) error {
	f, err := os.OpenFile(notesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening notes file: %w", err)
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("\n## Iteration %d (%s)\n%s\n", iteration, timestamp, notes)
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("writing notes: %w", err)
	}
	return nil
}

func acquireLock(path string) (bool, error) {
	for attempts := 0; attempts < 2; attempts++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return false, fmt.Errorf("writing lock pid: %w", err)
			}
			return true, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return false, fmt.Errorf("creating lock file %s: %w", path, err)
		}

		pid, err := readLockPID(path)
		if err != nil {
			return false, fmt.Errorf("lock file %s exists; another run may be active", path)
		}

		if isProcessRunning(pid) {
			return false, fmt.Errorf("lock file %s exists (pid %d); another run may be active", path, pid)
		}

		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("removing stale lock %s: %w", path, err)
		}
	}

	return false, fmt.Errorf("unable to acquire lock %s", path)
}

func readLockPID(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening lock file %s: %w", path, err)
	}
	defer f.Close()

	var pid int
	if _, err := fmt.Fscan(f, &pid); err != nil {
		return 0, fmt.Errorf("reading lock pid from %s: %w", path, err)
	}
	if pid <= 0 {
		return 0, fmt.Errorf("invalid lock pid %d", pid)
	}
	return pid, nil
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.ESRCH:
			return false
		case syscall.EPERM:
			return true
		}
	}

	return true
}

func releaseLock(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func installLockSignalHandler(lockPath string) func() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		select {
		case sig := <-c:
			signal.Stop(c)
			close(done)

			if err := releaseLock(lockPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to release lock: %v\n", err)
			}

			exitCode := 1
			switch sig {
			case syscall.SIGINT:
				exitCode = 130
			case syscall.SIGTERM:
				exitCode = 143
			}
			os.Exit(exitCode)
		case <-done:
			signal.Stop(c)
			return
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			signal.Stop(c)
			close(done)
		})
	}
}
