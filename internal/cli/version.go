package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use: "version", Aliases: []string{"ver", "ve", "v"},
		Short: "Print the application version", Args: withHelp(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", appName, version)
			return err
		},
	}
}
