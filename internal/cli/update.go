package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/naterator/voice-memos-transcript-extract/internal/update"
	"github.com/spf13/cobra"
)

type updateCheck func(context.Context, *http.Client, string, string) (update.Result, error)

func newUpdateCommand(version string, check updateCheck) *cobra.Command {
	if check == nil {
		check = update.Check
	}
	return &cobra.Command{
		Use: "update", Aliases: []string{"upd", "up", "u"},
		Short: "Check the latest stable GitHub release", Args: withHelp(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := check(cmd.Context(), &http.Client{Timeout: 10 * time.Second}, update.DefaultEndpoint, version)
			if err != nil {
				return err
			}
			var message string
			switch result.Status {
			case "newer":
				message = fmt.Sprintf("New release available: %s (current: %s)", result.Latest, result.Current)
			case "equal":
				message = fmt.Sprintf("Up to date: %s", result.Current)
			case "ahead":
				message = fmt.Sprintf("Current version %s is newer than latest release %s", result.Current, result.Latest)
			case "uncomparable":
				message = fmt.Sprintf("Current version: %s; latest release: %s", result.Current, result.Latest)
			default:
				return fmt.Errorf("update check returned an unknown status %q", result.Status)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", message, result.URL)
			return err
		},
	}
}
