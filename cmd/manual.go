package cmd

import (
	"github.com/spf13/cobra"

	"opencode-ralph/internal/ralph"
)

func newManualCmd(cfg ralph.Config) *cobra.Command {
	opts := &ralph.RunOptions{MaxIterations: 1}
	cmd := &cobra.Command{
		Use:          "manual",
		Short:        "Run exactly one iteration",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ralph.RunWithOptions(*opts, cfg.MaxIterations, cfg.MaxPerHour, cfg.MaxPerDay)
		},
	}
	bindRunFlags(cmd, cfg, opts)
	return cmd
}
