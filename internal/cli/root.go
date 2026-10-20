// Package cli builds the Voice Memos transcript extraction commands.
package cli

import (
	"github.com/spf13/cobra"
)

const appName = "voice-memos-transcript-extract"

// New builds a command with isolated flags and the supplied build version.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:               appName,
		Short:             "Extract embedded Apple Voice Memos transcripts",
		Version:           version,
		SilenceErrors:     true,
		SilenceUsage:      true,
		Args:              withHelp(cobra.NoArgs),
		RunE:              func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.SetVersionTemplate(appName + " {{.Version}}\n")
	root.SetFlagErrorFunc(showHelpOnError)
	root.AddCommand(newExtractCommand(), newCatCommand(), newVersionCommand(version), newUpdateCommand(version, nil))
	// Cobra gives its default version flag the -v shorthand.
	root.InitDefaultVersionFlag()
	return root
}

func withHelp(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return showHelpOnError(cmd, validate(cmd, args))
	}
}

func showHelpOnError(cmd *cobra.Command, err error) error {
	if err != nil {
		out := cmd.OutOrStdout()
		cmd.SetOut(cmd.ErrOrStderr())
		_ = cmd.Help()
		cmd.SetOut(out)
	}
	return err
}
