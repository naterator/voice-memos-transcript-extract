package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCatCommand() *cobra.Command {
	var track int
	cmd := &cobra.Command{
		Use: "cat FILE", Aliases: []string{"ca", "c"},
		Short: "Print the original transcript text",
		Args:  withHelp(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			selection, err := selectedTrack(cmd, track)
			if err != nil {
				return err
			}
			t, _, err := readTranscript(args[0], selection)
			if err != nil {
				return err
			}
			text := t.Text()
			if !strings.HasSuffix(text, "\n") {
				text += "\n"
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			return err
		},
	}
	cmd.Flags().IntVar(&track, "track", 0, "Select a zero-based track index")
	return cmd
}
