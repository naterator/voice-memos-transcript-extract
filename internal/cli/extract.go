package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/naterator/voice-memos-transcript-extract/internal/m4a"
	"github.com/naterator/voice-memos-transcript-extract/internal/stj"
	"github.com/naterator/voice-memos-transcript-extract/internal/transcript"
	"github.com/spf13/cobra"
)

func newExtractCommand() *cobra.Command {
	var output string
	var force bool
	var track int
	cmd := &cobra.Command{
		Use: "extract FILE", Aliases: []string{"ex", "e"},
		Short: "Write an embedded transcript as STJ JSON",
		Args:  withHelp(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			selection, err := selectedTrack(cmd, track)
			if err != nil {
				return err
			}
			t, inputInfo, err := readTranscript(args[0], selection)
			if err != nil {
				return err
			}
			document, warnings, err := stj.Convert(t, filepath.Base(args[0]))
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			var data bytes.Buffer
			encoder := json.NewEncoder(&data)
			encoder.SetIndent("", "  ")
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(document); err != nil {
				return fmt.Errorf("serialize transcript: %w", err)
			}
			destination := output
			if destination == "" {
				destination = defaultOutput(args[0])
			}
			if err := writeOutput(args[0], inputInfo, destination, data.Bytes(), force); err != nil {
				return err
			}
			for _, warning := range warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "notice:", warning)
			}
			_, err = fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", destination)
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output path (default: FILE.stj.json beside input)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Replace an existing regular output file")
	cmd.Flags().IntVar(&track, "track", 0, "Select a zero-based track index")
	return cmd
}

func selectedTrack(cmd *cobra.Command, track int) (*int, error) {
	if !cmd.Flags().Changed("track") {
		return nil, nil
	}
	if track < 0 {
		return nil, showHelpOnError(cmd, fmt.Errorf("--track must be nonnegative"))
	}
	return &track, nil
}

func defaultOutput(input string) string {
	extension := filepath.Ext(input)
	return strings.TrimSuffix(input, extension) + ".stj.json"
}

func readTranscript(path string, track *int) (transcript.Transcript, os.FileInfo, error) {
	var empty transcript.Transcript
	file, err := os.Open(path)
	if err != nil {
		return empty, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return empty, nil, fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return empty, nil, fmt.Errorf("%s: input must be a regular file", path)
	}
	t, readErr := m4a.Read(file, info.Size(), track)
	closeErr := file.Close()
	if readErr != nil {
		return empty, nil, fmt.Errorf("%s: %w", path, readErr)
	}
	if closeErr != nil {
		return empty, nil, fmt.Errorf("%s: %w", path, closeErr)
	}
	return t, info, nil
}
