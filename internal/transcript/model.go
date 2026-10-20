// Package transcript represents the timed text embedded by Apple Voice Memos.
package transcript

import "strings"

// TimeRange holds original start and end times in seconds.
type TimeRange struct {
	Start float64
	End   float64
}

// Run is one attributed text span; Timing is nil when timing is absent.
type Run struct {
	Text   string
	Timing *TimeRange
}

// Transcript is one track's embedded transcript, preserving source run order.
type Transcript struct {
	TrackIndex int
	Locale     string
	Runs       []Run
}

// Text reconstructs the transcript without modifying whitespace or punctuation.
func (t Transcript) Text() string {
	var b strings.Builder
	for _, run := range t.Runs {
		b.WriteString(run.Text)
	}
	return b.String()
}
