package cmd

import (
	"github.com/spf13/cobra"

	"opencode-ralph/internal/ralph"
)

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cfg := ralph.LoadConfig()
	opts := &ralph.RunOptions{}

	rootCmd := &cobra.Command{
		Use:           "opencode-ralph",
		Short:         "Iterative AI development orchestrator",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default behavior: same as `opencode-ralph run ...`
			return ralph.RunWithOptions(*opts, cfg.MaxIterations, cfg.MaxPerHour, cfg.MaxPerDay)
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

func bindRunFlags(cmd *cobra.Command, cfg ralph.Config, opts *ralph.RunOptions) {
	cmd.Flags().IntVar(&opts.MaxIterations, "max-iterations", cfg.MaxIterations, "Maximum iterations")
	cmd.Flags().IntVar(&opts.MaxPerHour, "max-per-hour", cfg.MaxPerHour, "Maximum iterations per hour (0 = unlimited)")
	cmd.Flags().IntVar(&opts.MaxPerDay, "max-per-day", cfg.MaxPerDay, "Maximum iterations per day (0 = unlimited)")
	cmd.Flags().StringVar(&opts.Prompt, "prompt", "", "Override prompt file path")
	cmd.Flags().StringVar(&opts.Conventions, "conventions", "", "Override conventions file path")
	cmd.Flags().StringVar(&opts.Specs, "specs", "", "Override specs file path")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "Agent to use (passed to opencode run --agent)")
	cmd.Flags().StringVar(&opts.Format, "format", "", "Output format (passed to opencode run --format; default|json)")
	cmd.Flags().BoolVar(&opts.ContinueSession, "continue", false, "Continue a previous session (passed to opencode run --continue)")
	cmd.Flags().StringVar(&opts.Session, "session", "", "Session ID (passed to opencode run --session)")
	cmd.Flags().StringArrayVar(&opts.Files, "file", nil, "File to attach (repeatable; passed to opencode run --file)")
	cmd.Flags().StringVar(&opts.Title, "title", "", "Message title (passed to opencode run --title)")
	cmd.Flags().StringVar(&opts.Variant, "variant", "", "Variant to use (passed to opencode run --variant)")
	cmd.Flags().StringVar(&opts.Attach, "attach", "", "Remote attach target (passed to opencode run --attach)")
	cmd.Flags().IntVar(&opts.Port, "port", 0, "Remote attach port (passed to opencode run --port)")
	cmd.Flags().BoolVar(&opts.Quiet, "quiet", false, "Hide opencode-ralph banner/status output")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Model to use (e.g., ollama/qwen3-coder:30b)")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Stream opencode output in real-time")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show constructed prompt without executing")
	cmd.Flags().Float64Var(&opts.Delay, "delay", 2.0, "Delay between iterations in seconds")
}
