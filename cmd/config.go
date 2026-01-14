package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"opencode-ralph/internal/ralph"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or modify configuration",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				out, err := ralph.ConfigView()
				if err != nil {
					return err
				}
				cmd.Println(out)
				return nil
			}

			switch args[0] {
			case "set":
				if len(args) < 3 {
					return fmt.Errorf("usage: config set KEY VALUE")
				}
				if err := ralph.ConfigSet(args[1], args[2]); err != nil {
					return err
				}
				cmd.Printf("Set %s = %s\n", args[1], args[2])
				return nil
			case "reset":
				if err := ralph.ConfigReset(); err != nil {
					return err
				}
				cmd.Println("Configuration reset to defaults")
				return nil
			default:
				return fmt.Errorf("unknown config command: %s", args[0])
			}
		},
	}
	return cmd
}
