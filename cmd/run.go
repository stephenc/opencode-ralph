package cmd

import (
	"github.com/spf13/cobra"

	"opencode-ralph/internal/ralph"
)

func newRunCmd(cfg ralph.Config) *cobra.Command {
	opts := &ralph.RunOptions{}
	cmd := &cobra.Command{
		Use:          "run",
		Short:        "Run multiple iterations until complete",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ralph.RunWithOptions(*opts, cfg.MaxIterations, cfg.MaxPerHour, cfg.MaxPerDay)
		},
	}
	bindRunFlags(cmd, cfg, opts)
	return cmd
}
