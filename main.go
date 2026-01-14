package main

import (
	"bytes"
	"embed"
	"encoding/json"
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

	"github.com/spf13/cobra"
)

//go:embed internal/ralph/templates/*
var templates embed.FS

// Config holds project configuration
type Config struct {
	PromptFile      string `json:"prompt_file"`
	ConventionsFile string `json:"conventions_file"`
	SpecsFile       string `json:"specs_file"`
	MaxIterations   int    `json:"max_iterations"`
	MaxPerHour      int    `json:"max_per_hour"`
	MaxPerDay       int    `json:"max_per_day"`
	Model           string `json:"model,omitempty"`
}

// State tracks iteration history for rate limiting
type State struct {
	TotalIterations int       `json:"total_iterations"`
	Timestamps      []int64   `json:"timestamps"`
	LastRun         time.Time `json:"last_run"`
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

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func defaultConfig() Config {
	return Config{
		PromptFile:      "PROMPT.md",
		ConventionsFile: "CONVENTIONS.md",
		SpecsFile:       "SPECS.md",
		MaxIterations:   50,
		MaxPerHour:      0,
		MaxPerDay:       0,
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type runOptions struct {
	maxIterations   int
	maxPerHour      int
	maxPerDay       int
	prompt          string
	conventions     string
	specs           string
	agent           string
	format          string
	continueSession bool
	session         string
	files           []string
	title           string
	variant         string
	attach          string
	port            int
	quiet           bool
	model           string
	verbose         bool
	dryRun          bool
	delay           float64
}

func newRootCmd() *cobra.Command {
	cfg := loadConfig()
	opts := &runOptions{}

	rootCmd := &cobra.Command{
		Use:           "opencode-ralph",
		Short:         "Iterative AI development orchestrator",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default behavior: same as `opencode-ralph run ...`
			return runWithOptions(*opts, cfg.MaxIterations, cfg.MaxPerHour, cfg.MaxPerDay)
		},
	}

	bindRunFlags(rootCmd, cfg, opts)

	legacyHelp := `opencode-ralph - Iterative AI development orchestrator

Usage:
  opencode-ralph [command] [options]

Commands:
  init      Create PROMPT.md, CONVENTIONS.md, and stub SPECS.md
  manual    Run exactly one iteration
  run       Run multiple iterations until complete (default)
  config    View or modify configuration
  help      Show this help message

Run Options:
  --max-iterations N    Maximum iterations (default: from config or 50)
  --max-per-hour N      Maximum iterations per hour (default: from config or 0)
  --max-per-day N       Maximum iterations per day (default: from config or 0)
  --prompt FILE         Override prompt file path
  --conventions FILE    Override conventions file path
  --specs FILE          Override specs file path
  --agent AGENT         Agent to use (passed to opencode run --agent)
  --format FORMAT       Output format (passed to opencode run --format; default|json)
  --continue            Continue a previous session (passed to opencode run --continue)
  --session SESSION     Session ID (passed to opencode run --session)
  --file FILE           Attach file (repeatable; passed to opencode run --file)
  --title TITLE         Message title (passed to opencode run --title)
  --variant VARIANT     Variant to use (passed to opencode run --variant)
  --attach ATTACH       Remote attach target (passed to opencode run --attach)
  --port PORT           Remote attach port (passed to opencode run --port)
  --quiet               Hide opencode-ralph banner/status output
  --model MODEL         Model to use (e.g., ollama/qwen3-coder:30b)
  --verbose             Stream opencode output in real-time
  --dry-run             Show constructed prompt without executing
  --delay SECONDS       Delay between iterations (default: 2s)


Config Commands:
  config                Show current configuration
  config set KEY VALUE  Set a configuration value
  config reset          Reset configuration to defaults

Config Keys:
  prompt_file, conventions_file, specs_file,
  max_iterations, max_per_hour, max_per_day, model

Examples:
  opencode-ralph init
  opencode-ralph manual --verbose
  opencode-ralph run --max-iterations 10
  opencode-ralph config set specs_file TASKS.md
  opencode-ralph --specs TASKS.md --max-per-hour 5
`

	rootCmd.SetHelpTemplate(legacyHelp)

	// Override cobra's default help/usage rendering to keep legacy output.
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.Println(legacyHelp)
	})
	rootCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		cmd.Println(legacyHelp)
		return nil
	})

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newManualCmd(cfg))
	rootCmd.AddCommand(newRunCmd(cfg))
	rootCmd.AddCommand(newConfigCmd())

	return rootCmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create PROMPT.md, CONVENTIONS.md, and stub SPECS.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			initCmd()
			return nil
		},
	}
}

func newManualCmd(cfg Config) *cobra.Command {
	opts := &runOptions{maxIterations: 1}
	cmd := &cobra.Command{
		Use:          "manual",
		Short:        "Run exactly one iteration",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithOptions(*opts, cfg.MaxIterations, cfg.MaxPerHour, cfg.MaxPerDay)
		},
	}
	bindRunFlags(cmd, cfg, opts)
	return cmd
}

func newRunCmd(cfg Config) *cobra.Command {
	opts := &runOptions{}
	cmd := &cobra.Command{
		Use:          "run",
		Short:        "Run multiple iterations until complete",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithOptions(*opts, cfg.MaxIterations, cfg.MaxPerHour, cfg.MaxPerDay)
		},
	}
	bindRunFlags(cmd, cfg, opts)
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or modify configuration",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configCmd(args)
			return nil
		},
	}
	return cmd
}

func initCmd() {

	// Create .ralph directory
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .ralph directory: %v\n", err)
		os.Exit(1)
	}

	// Load or create config
	cfg := loadConfig()

	// Create PROMPT.md if not exists
	if err := createFromTemplate(cfg.PromptFile, "internal/ralph/templates/PROMPT.md"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create CONVENTIONS.md if not exists
	if err := createFromTemplate(cfg.ConventionsFile, "internal/ralph/templates/CONVENTIONS.md"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create SPECS.md if not exists
	if err := createFromTemplate(cfg.SpecsFile, "internal/ralph/templates/SPECS.md"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Save default config if it doesn't exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		saveConfig(cfg)
		fmt.Println("Created .ralph/config.json")
	}

	fmt.Printf("\nInitialization complete. Edit %s to define your tasks.\n", cfg.SpecsFile)
}

func createFromTemplate(destPath, templatePath string) error {
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		fmt.Printf("%s already exists, skipping\n", destPath)
		return nil
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

func configCmd(args []string) {
	if len(args) == 0 {
		// Show current config
		cfg := loadConfig()
		data, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(data))
		return
	}

	switch args[0] {
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: config set KEY VALUE")
			os.Exit(1)
		}
		configSet(args[1], args[2])
	case "reset":
		cfg := defaultConfig()
		saveConfig(cfg)
		fmt.Println("Configuration reset to defaults")
	default:
		fmt.Fprintf(os.Stderr, "Unknown config command: %s\n", args[0])
		os.Exit(1)
	}
}

func configSet(key, value string) {
	cfg := loadConfig()

	switch key {
	case "prompt_file":
		cfg.PromptFile = value
	case "conventions_file":
		cfg.ConventionsFile = value
	case "specs_file":
		cfg.SpecsFile = value
	case "max_iterations":
		var v int
		fmt.Sscanf(value, "%d", &v)
		cfg.MaxIterations = v
	case "max_per_hour":
		var v int
		fmt.Sscanf(value, "%d", &v)
		cfg.MaxPerHour = v
	case "max_per_day":
		var v int
		fmt.Sscanf(value, "%d", &v)
		cfg.MaxPerDay = v
	case "model":
		cfg.Model = value
	default:
		fmt.Fprintf(os.Stderr, "Unknown config key: %s\n", key)
		os.Exit(1)
	}

	saveConfig(cfg)
	fmt.Printf("Set %s = %s\n", key, value)
}

func runWithOptions(opts runOptions, defaultMaxIterations, defaultMaxPerHour, defaultMaxPerDay int) error {
	cfg := loadConfig()

	maxIterations := opts.maxIterations
	if maxIterations == 0 {
		maxIterations = defaultMaxIterations
	}

	maxPerHour := opts.maxPerHour
	if maxPerHour == 0 {
		maxPerHour = defaultMaxPerHour
	}

	maxPerDay := opts.maxPerDay
	if maxPerDay == 0 {
		maxPerDay = defaultMaxPerDay
	}

	if opts.prompt != "" {
		cfg.PromptFile = opts.prompt
	}
	if opts.conventions != "" {
		cfg.ConventionsFile = opts.conventions
	}
	if opts.specs != "" {
		cfg.SpecsFile = opts.specs
	}

	modelToUse := opts.model
	if modelToUse == "" {
		modelToUse = cfg.Model
	}

	if opts.format != "" && opts.format != "default" && opts.format != "json" {
		return fmt.Errorf("invalid --format value: %s (expected default or json)", opts.format)
	}
	if opts.continueSession && opts.session != "" {
		return fmt.Errorf("invalid flags: --continue and --session are mutually exclusive")
	}

	quietFlag := opts.quiet
	if opts.dryRun {
		quietFlag = false
	}

	verboseFlag := opts.verbose || quietFlag
	if opts.dryRun {
		verboseFlag = false
	}

	return runIterations(cfg, maxIterations, maxPerHour, maxPerDay, modelToUse, opts.agent, opts.format, opts.variant, opts.attach, opts.port, opts.continueSession, opts.session, stringSliceFlag(opts.files), opts.title, quietFlag, verboseFlag, opts.dryRun, opts.delay)
}

func bindRunFlags(cmd *cobra.Command, cfg Config, opts *runOptions) {
	cmd.Flags().IntVar(&opts.maxIterations, "max-iterations", cfg.MaxIterations, "Maximum iterations")
	cmd.Flags().IntVar(&opts.maxPerHour, "max-per-hour", cfg.MaxPerHour, "Maximum iterations per hour (0 = unlimited)")
	cmd.Flags().IntVar(&opts.maxPerDay, "max-per-day", cfg.MaxPerDay, "Maximum iterations per day (0 = unlimited)")
	cmd.Flags().StringVar(&opts.prompt, "prompt", "", "Override prompt file path")
	cmd.Flags().StringVar(&opts.conventions, "conventions", "", "Override conventions file path")
	cmd.Flags().StringVar(&opts.specs, "specs", "", "Override specs file path")
	cmd.Flags().StringVar(&opts.agent, "agent", "", "Agent to use (passed to opencode run --agent)")
	cmd.Flags().StringVar(&opts.format, "format", "", "Output format (passed to opencode run --format; default|json)")
	cmd.Flags().BoolVar(&opts.continueSession, "continue", false, "Continue a previous session (passed to opencode run --continue)")
	cmd.Flags().StringVar(&opts.session, "session", "", "Session ID (passed to opencode run --session)")
	cmd.Flags().StringArrayVar(&opts.files, "file", nil, "File to attach (repeatable; passed to opencode run --file)")
	cmd.Flags().StringVar(&opts.title, "title", "", "Message title (passed to opencode run --title)")
	cmd.Flags().StringVar(&opts.variant, "variant", "", "Variant to use (passed to opencode run --variant)")
	cmd.Flags().StringVar(&opts.attach, "attach", "", "Remote attach target (passed to opencode run --attach)")
	cmd.Flags().IntVar(&opts.port, "port", 0, "Remote attach port (passed to opencode run --port)")
	cmd.Flags().BoolVar(&opts.quiet, "quiet", false, "Hide opencode-ralph banner/status output")
	cmd.Flags().StringVar(&opts.model, "model", "", "Model to use (e.g., ollama/qwen3-coder:30b)")
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "Stream opencode output in real-time")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Show constructed prompt without executing")
	cmd.Flags().Float64Var(&opts.delay, "delay", 2.0, "Delay between iterations in seconds")
}

func runIterations(cfg Config, maxIterations, maxPerHour, maxPerDay int, model string, agent string, format string, variant string, attach string, port int, continueSession bool, session string, files stringSliceFlag, title string, quiet bool, verbose, dryRun bool, delay float64) (err error) {
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

	// Ensure .ralph directory exists
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return fmt.Errorf("creating .ralph directory: %w", err)
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

		// Check rate limits
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

		// Load all files
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

		// Construct prompt
		prompt := constructPrompt(promptMD, conventionsMD, specsMD, notesMD, iteration, maxIterations)

		if dryRun {
			fmt.Println("\n--- DRY RUN: Constructed Prompt ---")
			fmt.Println(prompt)
			fmt.Println("--- END DRY RUN ---")
			finalStatus = "dry_run"
			return nil
		}

		// Run opencode
		output, err := runOpencode(prompt, model, agent, format, variant, attach, port, continueSession, session, files, title, quiet, verbose)
		if err != nil {
			if !quiet {
				fmt.Printf("%s\n", styleIf(useColor, fmt.Sprintf("Warning: opencode exited with error: %v", err), ansiYellow, ansiBold))
			}
			// If opencode fails, we still want to continue processing notes
			// but don't treat this as an error that stops the iteration
		}

		// Extract and save notes
		if notes := extractNotes(output); notes != "" {
			if err := appendNotes(notes, iteration); err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "Warning: failed to save notes: %v\n", err)

				}
			}
		}

		// Check for completion signal
		if isComplete(output) {
			finalStatus = "complete"
			if !quiet {
				fmt.Println(styleIf(useColor, "Received COMPLETE signal from opencode!", ansiGreen, ansiBold))
			}
			return nil
		}

		// Record this iteration's timestamp
		state.Timestamps = append(state.Timestamps, time.Now().Unix())
		state.LastRun = time.Now()
		pruneOldTimestamps(&state)
		saveState(state)

		// Delay between iterations
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

func loadConfig() Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(configFile)
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) {
	os.MkdirAll(ralphDir, 0755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configFile, data, 0644)
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
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(stateFile, data, 0644)
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

		if !os.IsExist(err) {
			return false, fmt.Errorf("creating lock file %s: %w", path, err)
		}

		pid, err := readLockPID(path)
		if err != nil {
			return false, fmt.Errorf("lock file %s exists; another run may be active", path)
		}

		if isProcessRunning(pid) {
			return false, fmt.Errorf("lock file %s exists (pid %d); another run may be active", path, pid)
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
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

	// If we can't reliably determine, treat as running.
	return true
}

func releaseLock(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
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

func runOpencode(prompt string, model string, agent string, format string, variant string, attach string, port int, continueSession bool, session string, files stringSliceFlag, title string, quiet bool, verbose bool) (string, error) {
	args := []string{"run"}
	if model != "" {
		args = append(args, "-m", model)
	}
	if agent != "" {
		args = append(args, "--agent", agent)
	}
	if format != "" {
		args = append(args, "--format", format)
	}
	if variant != "" {
		args = append(args, "--variant", variant)
	}
	if attach != "" {
		args = append(args, "--attach", attach)
	}
	if port != 0 {
		args = append(args, "--port", fmt.Sprintf("%d", port))
	}
	if continueSession {
		args = append(args, "--continue")
	}
	if session != "" {
		args = append(args, "--session", session)
	}
	for _, file := range files {
		if file != "" {
			args = append(args, "--file", file)
		}
	}
	if title != "" {
		args = append(args, "--title", title)
	}
	args = append(args, prompt)
	cmd := exec.Command("opencode", args...)

	var output bytes.Buffer

	if verbose || quiet {
		cmd.Stdout = io.MultiWriter(os.Stdout, &output)
		cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	} else {
		cmd.Stdout = &output
		cmd.Stderr = &output
	}

	err := cmd.Run()

	// Check if the error is due to a non-zero exit code
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// If opencode exits with non-zero code, we should treat this as an error
			// but still capture the output for notes extraction
			return output.String(), err
		}
		// For other types of errors, return them as is
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
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("\n## Iteration %d (%s)\n%s\n", iteration, timestamp, notes)
	_, err = f.WriteString(entry)
	return err
}
