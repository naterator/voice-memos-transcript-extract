// Package stj converts embedded Voice Memos transcripts to STJ 0.6.1.
package stj

// Document is the JSON root defined by STJ.
type Document struct {
	STJ Content `json:"stj"`
}

// Content contains the version, metadata, and transcription segments.
type Content struct {
	Version    string        `json:"version"`
	Metadata   Metadata      `json:"metadata"`
	Transcript Transcription `json:"transcript"`
}

type Metadata struct {
	Transcriber Transcriber `json:"transcriber"`
	Extensions  Extensions  `json:"extensions"`
}

type Transcriber struct {
	Name string `json:"name"`
}

type Extensions struct {
	AppleVoiceMemos AppleVoiceMemos `json:"apple_voice_memos"`
}

// AppleVoiceMemos preserves source run boundaries and durations outside the
// standard STJ fields. Durations are derived from rounded start/end values.
type AppleVoiceMemos struct {
	SourceFile       string      `json:"source_file"`
	TrackIndex       int         `json:"track_index"`
	LocaleIdentifier string      `json:"locale_identifier,omitempty"`
	Runs             []SourceRun `json:"runs"`
}

type SourceRun struct {
	Text            string   `json:"text"`
	Start           *float64 `json:"start,omitempty"`
	End             *float64 `json:"end,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
}

type Transcription struct {
	Segments []Segment `json:"segments"`
}

type Segment struct {
	Start          *float64 `json:"start,omitempty"`
	End            *float64 `json:"end,omitempty"`
	IsZeroDuration bool     `json:"is_zero_duration,omitempty"`
	Text           string   `json:"text"`
	WordTimingMode string   `json:"word_timing_mode,omitempty"`
	Words          []Word   `json:"words,omitempty"`
}

// Word holds one source-timed span, which may contain several lexical words.
type Word struct {
	Text           string  `json:"text"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	IsZeroDuration bool    `json:"is_zero_duration,omitempty"`
}
