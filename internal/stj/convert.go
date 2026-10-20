package stj

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/naterator/voice-memos-transcript-extract/internal/transcript"
)

const maxMilliseconds = 999999999

// Convert maps source runs to one STJ segment without changing their text or
// inventing word boundaries. Times are rounded once to integer milliseconds.
func Convert(t transcript.Transcript, sourceName string) (Document, []string, error) {
	text := t.Text()
	if strings.TrimSpace(text) == "" {
		return Document{}, nil, fmt.Errorf("empty transcript")
	}

	segment := Segment{Text: text}
	source := AppleVoiceMemos{
		SourceFile:       filepath.Base(sourceName),
		TrackIndex:       t.TrackIndex,
		LocaleIdentifier: t.Locale,
		Runs:             make([]SourceRun, 0, len(t.Runs)),
	}
	var timedText strings.Builder
	var firstMS, lastMS int64
	var previousStart float64
	timedCount, textualCount := 0, 0

	for i, run := range t.Runs {
		original := SourceRun{Text: run.Text}
		textual := strings.TrimSpace(run.Text) != ""
		if textual {
			textualCount++
		}
		if run.Timing != nil {
			startMS, endMS, err := milliseconds(*run.Timing)
			if err != nil {
				return Document{}, nil, fmt.Errorf("track %d run %d: %w", t.TrackIndex, i, err)
			}
			original.Start = seconds(startMS)
			original.End = seconds(endMS)
			original.DurationSeconds = seconds(endMS - startMS)
			if textual {
				if timedCount > 0 && run.Timing.Start < previousStart {
					return Document{}, nil, fmt.Errorf("track %d run %d: unsupported non-monotonic timing (start decreases)", t.TrackIndex, i)
				}
				previousStart = run.Timing.Start
				if timedCount == 0 || startMS < firstMS {
					firstMS = startMS
				}
				if timedCount == 0 || endMS > lastMS {
					lastMS = endMS
				}
				segment.Words = append(segment.Words, Word{
					Text:           run.Text,
					Start:          *original.Start,
					End:            *original.End,
					IsZeroDuration: startMS == endMS,
				})
				timedText.WriteString(run.Text)
				timedCount++
			}
		}
		source.Runs = append(source.Runs, original)
	}

	var warnings []string
	switch {
	case timedCount == 0:
		segment.WordTimingMode = "none"
		warnings = append(warnings, "word timing is unavailable")
	case timedCount == textualCount && normalized(timedText.String()) == normalized(text):
		segment.WordTimingMode = "complete"
	default:
		segment.WordTimingMode = "partial"
		warnings = append(warnings, "word timing is partial")
	}
	if timedCount > 0 {
		segment.Start, segment.End = seconds(firstMS), seconds(lastMS)
		if firstMS == lastMS {
			segment.IsZeroDuration = true
			segment.Words = nil
			segment.WordTimingMode = ""
		}
	}

	return Document{STJ: Content{
		Version: "0.6.1",
		Metadata: Metadata{
			Transcriber: Transcriber{Name: "Apple Voice Memos"},
			Extensions:  Extensions{AppleVoiceMemos: source},
		},
		Transcript: Transcription{Segments: []Segment{segment}},
	}}, warnings, nil
}

func milliseconds(t transcript.TimeRange) (int64, int64, error) {
	if math.IsNaN(t.Start) || math.IsNaN(t.End) || math.IsInf(t.Start, 0) || math.IsInf(t.End, 0) || t.Start < 0 || t.End < t.Start {
		return 0, 0, fmt.Errorf("invalid timing: require finite nonnegative start <= end")
	}
	startMS := math.RoundToEven(t.Start * 1000)
	endMS := math.RoundToEven(t.End * 1000)
	if startMS > maxMilliseconds || endMS > maxMilliseconds {
		return 0, 0, fmt.Errorf("timing exceeds STJ maximum of 999999.999 seconds")
	}
	return int64(startMS), int64(endMS), nil
}

func seconds(ms int64) *float64 {
	value := float64(ms) / 1000
	return &value
}

func normalized(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
