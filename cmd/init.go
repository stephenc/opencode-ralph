package cmd

import (
	"github.com/spf13/cobra"

	"opencode-ralph/internal/ralph"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create PROMPT.md, CONVENTIONS.md, and stub SPECS.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ralph.Init()
		},
	}
}
