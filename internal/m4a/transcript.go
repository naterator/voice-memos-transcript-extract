package m4a

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/naterator/voice-memos-transcript-extract/internal/transcript"
)

// Decode only the documented Apple attributed-string layouts. RawMessage keeps
// type checks explicit: JSON null must not silently become an empty Go value.
func decode(data []byte) (transcript.Transcript, error) {
	var t transcript.Transcript
	if !utf8.Valid(data) {
		return t, fmt.Errorf("invalid transcript UTF-8")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return t, fmt.Errorf("invalid transcript JSON: %w", err)
	}
	if root == nil {
		return t, fmt.Errorf("unsupported Apple transcript layout: expected a JSON object")
	}
	value := bytes.TrimSpace(root["attributedString"])
	if len(value) == 0 {
		return t, fmt.Errorf("unsupported Apple transcript layout: missing attributedString")
	}
	var err error
	switch value[0] {
	case '[':
		t.Runs, err = inlineRuns(value)
	case '{':
		t.Runs, err = indexedRuns(value)
	default:
		err = fmt.Errorf("unsupported Apple attributedString layout: expected array or indexed object")
	}
	if err != nil {
		return t, err
	}
	if value, ok := root["locale"]; ok {
		locale, err := object(value)
		if err != nil {
			return t, fmt.Errorf("locale: %w", err)
		}
		if identifier, ok := locale["identifier"]; ok {
			t.Locale, err = stringValue(identifier)
			if err != nil {
				return t, fmt.Errorf("locale identifier: %w", err)
			}
		}
	}
	if strings.TrimSpace(t.Text()) == "" {
		return t, fmt.Errorf("empty transcript: text contains no non-whitespace content")
	}
	return t, nil
}

func inlineRuns(value json.RawMessage) ([]transcript.Run, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(value, &items); err != nil {
		return nil, fmt.Errorf("attributedString array: %w", err)
	}
	if len(items)%2 != 0 {
		return nil, fmt.Errorf("attributedString array: expected text/attribute pairs, got %d items", len(items))
	}
	runs := make([]transcript.Run, 0, len(items)/2)
	for i := 0; i < len(items); i += 2 {
		text, err := stringValue(items[i])
		if err != nil {
			return nil, fmt.Errorf("run %d text: %w", i/2, err)
		}
		timing, err := attribute(items[i+1])
		if err != nil {
			return nil, fmt.Errorf("run %d attribute: %w", i/2, err)
		}
		runs = append(runs, transcript.Run{Text: text, Timing: timing})
	}
	return runs, nil
}

func indexedRuns(value json.RawMessage) ([]transcript.Run, error) {
	fields, err := object(value)
	if err != nil {
		return nil, err
	}
	var items, table []json.RawMessage
	if err := array(fields["runs"], &items); err != nil {
		return nil, fmt.Errorf("unsupported Apple attributedString object runs: %w", err)
	}
	if err := array(fields["attributeTable"], &table); err != nil {
		return nil, fmt.Errorf("unsupported Apple attributedString object attributeTable: %w", err)
	}
	if len(items)%2 != 0 {
		return nil, fmt.Errorf("runs: expected text/index pairs, got %d items", len(items))
	}
	attributes := make([]*transcript.TimeRange, len(table))
	for i, raw := range table {
		attributes[i], err = attribute(raw)
		if err != nil {
			return nil, fmt.Errorf("attribute %d: %w", i, err)
		}
	}
	runs := make([]transcript.Run, 0, len(items)/2)
	for i := 0; i < len(items); i += 2 {
		text, err := stringValue(items[i])
		if err != nil {
			return nil, fmt.Errorf("run %d text: %w", i/2, err)
		}
		var index int
		raw := bytes.TrimSpace(items[i+1])
		if len(raw) == 0 || raw[0] < '0' || raw[0] > '9' {
			return nil, fmt.Errorf("run %d attribute index: expected a nonnegative integer", i/2)
		}
		if err := json.Unmarshal(raw, &index); err != nil {
			return nil, fmt.Errorf("run %d attribute index: expected a nonnegative integer: %w", i/2, err)
		}
		if index >= len(attributes) {
			return nil, fmt.Errorf("run %d attribute index %d is out of range (attributeTable has %d entries)", i/2, index, len(attributes))
		}
		runs = append(runs, transcript.Run{Text: text, Timing: attributes[index]})
	}
	return runs, nil
}

func object(raw json.RawMessage) (map[string]json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("expected an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func array(raw json.RawMessage, values *[]json.RawMessage) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return fmt.Errorf("expected an array")
	}
	return json.Unmarshal(raw, values)
}

func stringValue(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' {
		return "", fmt.Errorf("expected a string")
	}
	var text string
	err := json.Unmarshal(raw, &text)
	return text, err
}

func attribute(raw json.RawMessage) (*transcript.TimeRange, error) {
	fields, err := object(raw)
	if err != nil {
		return nil, err
	}
	value, ok := fields["timeRange"]
	if !ok {
		return nil, nil
	}
	var pair []json.RawMessage
	if err := array(value, &pair); err != nil || len(pair) != 2 {
		return nil, fmt.Errorf("timeRange must contain exactly two numbers [start, end]")
	}
	var times [2]float64
	for i, raw := range pair {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || (raw[0] != '-' && (raw[0] < '0' || raw[0] > '9')) {
			return nil, fmt.Errorf("timeRange value %d must be a finite nonnegative number", i)
		}
		if err := json.Unmarshal(raw, &times[i]); err != nil || math.IsNaN(times[i]) || math.IsInf(times[i], 0) || times[i] < 0 {
			return nil, fmt.Errorf("timeRange value %d must be a finite nonnegative number", i)
		}
	}
	if times[0] > times[1] {
		return nil, fmt.Errorf("timeRange start exceeds end")
	}
	return &transcript.TimeRange{Start: times[0], End: times[1]}, nil
}
