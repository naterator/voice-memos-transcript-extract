package stj

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/naterator/voice-memos-transcript-extract/internal/transcript"
)

func timed(text string, start, end float64) transcript.Run {
	return transcript.Run{Text: text, Timing: &transcript.TimeRange{Start: start, End: end}}
}

func TestConvertExpectedOutput(t *testing.T) {
	doc, warnings, err := Convert(transcript.Transcript{
		TrackIndex: 2,
		Locale:     "en_US",
		Runs: []transcript.Run{
			{Text: "\n"},
			timed("Hello", 0.2505, 0.7505),
			timed(" world again.", 0.8005, 1.4005),
			{Text: "\t"},
		},
	}, "/private/recordings/Meeting.m4a")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	got, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
		"stj": {
			"version": "0.6.1",
			"metadata": {
				"transcriber": {"name": "Apple Voice Memos"},
				"extensions": {"apple_voice_memos": {
					"source_file": "Meeting.m4a", "track_index": 2, "locale_identifier": "en_US",
					"runs": [
						{"text": "\n"},
						{"text": "Hello", "start": 0.25, "end": 0.75, "duration_seconds": 0.5},
						{"text": " world again.", "start": 0.8, "end": 1.4, "duration_seconds": 0.6},
						{"text": "\t"}
					]
				}}
			},
			"transcript": {"segments": [{
				"start": 0.25, "end": 1.4, "text": "\nHello world again.\t", "word_timing_mode": "complete",
				"words": [
					{"text": "Hello", "start": 0.25, "end": 0.75},
					{"text": " world again.", "start": 0.8, "end": 1.4}
				]
			}]}
		}
	}`
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(want)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, compact.Bytes()) {
		t.Fatalf("unexpected JSON\ngot:  %s\nwant: %s", got, compact.Bytes())
	}
}

func TestConvertTimingModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		runs []transcript.Run
		mode string
	}{
		{"untimed text", []transcript.Run{timed("Hello", 1, 2), {Text: " world"}}, "partial"},
		{"standalone whitespace", []transcript.Run{timed("Hello", 1, 2), {Text: " "}, timed("world", 3, 4)}, "partial"},
		{"no timing", []transcript.Run{{Text: "Hello world"}}, "none"},
		{"whitespace timing only", []transcript.Run{timed(" ", 1, 2), {Text: "Hello"}}, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := transcript.Transcript{Runs: tc.runs}
			doc, warnings, err := Convert(input, "sample.m4a")
			if err != nil {
				t.Fatal(err)
			}
			segment := doc.STJ.Transcript.Segments[0]
			if segment.Text != input.Text() || segment.WordTimingMode != tc.mode || len(warnings) != 1 {
				t.Fatalf("segment = %+v, warnings = %v", segment, warnings)
			}
			if tc.mode == "none" && (segment.Start != nil || segment.End != nil || len(segment.Words) != 0) {
				t.Fatalf("untimed segment has timing: %+v", segment)
			}
		})
	}
}

func TestConvertRoundedZeroDurations(t *testing.T) {
	for _, tc := range []struct {
		name string
		runs []transcript.Run
		zero bool
	}{
		{"zero segment", []transcript.Run{timed("Hello", 1.0001, 1.0004)}, true},
		{"zero word", []transcript.Run{timed("Hello", 0.0005, 0.0005), timed(" world", 0.0015, 0.1)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, _, err := Convert(transcript.Transcript{Runs: tc.runs}, "sample.m4a")
			if err != nil {
				t.Fatal(err)
			}
			segment := doc.STJ.Transcript.Segments[0]
			if segment.IsZeroDuration != tc.zero {
				t.Fatalf("zero flag = %v", segment.IsZeroDuration)
			}
			encoded, err := json.Marshal(segment)
			if err != nil {
				t.Fatal(err)
			}
			if tc.zero {
				if string(encoded) != `{"start":1,"end":1,"is_zero_duration":true,"text":"Hello"}` {
					t.Fatalf("zero segment JSON: %s", encoded)
				}
			} else if !segment.Words[0].IsZeroDuration || segment.Words[0].Start != 0 || segment.Words[1].Start != 0.002 {
				t.Fatalf("rounded words: %+v", segment.Words)
			}
			if duration := *doc.STJ.Metadata.Extensions.AppleVoiceMemos.Runs[0].DurationSeconds; duration != 0 {
				t.Fatalf("zero source duration = %v", duration)
			}
		})
	}
}

func TestConvertTimingValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		runs []transcript.Run
		want string
	}{
		{"backwards", []transcript.Run{timed("Hello", 1, 2), timed(" world", 0.9999, 3)}, "non-monotonic"},
		{"range", []transcript.Run{timed("Hello", 0, 1000000)}, "STJ maximum"},
		{"nan", []transcript.Run{timed("Hello", math.NaN(), 1)}, "invalid timing"},
		{"negative", []transcript.Run{timed("Hello", -1, 1)}, "invalid timing"},
		{"reversed", []transcript.Run{timed("Hello", 2, 1)}, "invalid timing"},
		{"empty", []transcript.Run{{Text: "\t\n "}}, "empty transcript"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Convert(transcript.Transcript{Runs: tc.runs}, "sample.m4a")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	doc, _, err := Convert(transcript.Transcript{Runs: []transcript.Run{timed("Hello", 0, 2), timed(" world", 1, 1.5)}}, "sample.m4a")
	if err != nil || *doc.STJ.Transcript.Segments[0].End != 2 {
		t.Fatalf("overlap: document = %+v, error = %v", doc, err)
	}
	doc, _, err = Convert(transcript.Transcript{Runs: []transcript.Run{timed("Hello", 0, 999999.9994)}}, "sample.m4a")
	if err != nil || *doc.STJ.Transcript.Segments[0].End != 999999.999 {
		t.Fatalf("rounded maximum: document = %+v, error = %v", doc, err)
	}
}
