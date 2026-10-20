package m4a

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/naterator/voice-memos-transcript-extract/internal/transcript"
)

func testBox(kind string, payload ...[]byte) []byte {
	data := bytes.Join(payload, nil)
	result := make([]byte, 8+len(data))
	binary.BigEndian.PutUint32(result, uint32(len(result)))
	copy(result[4:8], kind)
	copy(result[8:], data)
	return result
}

func testExtendedBox(kind string, data []byte) []byte {
	result := make([]byte, 16+len(data))
	binary.BigEndian.PutUint32(result, 1)
	copy(result[4:8], kind)
	binary.BigEndian.PutUint64(result[8:16], uint64(len(result)))
	copy(result[16:], data)
	return result
}

func testTrack(payload string) []byte {
	return testBox("trak", testBox("udta", testBox("tsrp", []byte(payload))))
}

func TestReadAppleLayouts(t *testing.T) {
	forms := map[string]string{
		"inline":  `{"attributedString":["Hello",{"timeRange":[0.25,0.75]}," world.",{"timeRange":[0.8,1.4]}],"locale":{"identifier":"en_US"}}`,
		"indexed": `{"attributedString":{"runs":["Hello",0," world.",1],"attributeTable":[{"timeRange":[0.25,0.75]},{"timeRange":[0.8,1.4]}]},"locale":{"identifier":"en_US"}}`,
	}
	want := transcript.Transcript{
		TrackIndex: 1,
		Locale:     "en_US",
		Runs: []transcript.Run{
			{Text: "Hello", Timing: &transcript.TimeRange{Start: 0.25, End: 0.75}},
			{Text: " world.", Timing: &transcript.TimeRange{Start: 0.8, End: 1.4}},
		},
	}
	for name, payload := range forms {
		t.Run(name, func(t *testing.T) {
			// Track ordinals include the first track without metadata. The zero-
			// size udta extends to its parent's end, and tsrp uses a 64-bit size.
			udta := testBox("udta", testExtendedBox("tsrp", []byte(payload)))
			binary.BigEndian.PutUint32(udta, 0)
			data := testBox("moov", testBox("trak", testBox("free")), testBox("trak", udta))
			data = append(data, testBox("mdat", []byte("tsrp is not metadata here"))...)
			got, err := Read(bytes.NewReader(data), int64(len(data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v; want %#v", got, want)
			}
		})
	}
}

func TestReadErrors(t *testing.T) {
	missing := testBox("moov", testBox("trak"))
	truncated := append(testBox("moov"), 0, 0, 0)
	cases := []struct {
		name         string
		data         []byte
		want         string
		noTranscript bool
	}{
		{"missing transcript", missing, "no embedded Voice Memos transcript", true},
		{"missing moov", testBox("mdat"), "no moov", false},
		{"truncated box", truncated, "truncated box header", false},
		{"invalid index", testBox("moov", testTrack(`{"attributedString":{"runs":["Hello",2],"attributeTable":[{}]}}`)), "track 0: run 0 attribute index 2 is out of range", false},
		{"invalid range", testBox("moov", testTrack(`{"attributedString":["Hello",{"timeRange":[0,null]}]}`)), "track 0: run 0 attribute: timeRange value 1", false},
		{"invalid attribute", testBox("moov", testTrack(`{"attributedString":["Hello",null]}`)), "run 0 attribute: expected an object", false},
		{"empty transcript", testBox("moov", testTrack(`{"attributedString":[" \n",{}]}`)), "empty transcript", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Read(bytes.NewReader(tc.data), int64(len(tc.data)), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v; want error containing %q", err, tc.want)
			}
			if errors.Is(err, ErrNoTranscript) != tc.noTranscript {
				t.Fatalf("ErrNoTranscript match = %v; want %v", errors.Is(err, ErrNoTranscript), tc.noTranscript)
			}
		})
	}
}

func TestReadTrackSelection(t *testing.T) {
	valid := `{"attributedString":[" Héllo",{},"",{}," world. ",{}]}`
	data := testBox("moov", testTrack(`broken JSON`), testTrack(valid))
	_, err := Read(bytes.NewReader(data), int64(len(data)), nil)
	if err == nil || !strings.Contains(err.Error(), "available track indices: 0, 1") {
		t.Fatalf("expected track ambiguity before JSON decoding; got %v", err)
	}
	selected := 1
	got, err := Read(bytes.NewReader(data), int64(len(data)), &selected)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text() != " Héllo world. " || len(got.Runs) != 3 || got.Runs[0].Timing != nil {
		t.Fatalf("untimed, empty or Unicode run text changed: %#v", got)
	}
	data = testBox("moov", testBox("trak", testBox("udta", testBox("tsrp", []byte(valid)), testBox("tsrp", []byte(valid)))))
	_, err = Read(bytes.NewReader(data), int64(len(data)), nil)
	if err == nil || !strings.Contains(err.Error(), "found 2 tsrp boxes") {
		t.Fatalf("expected duplicate payload ambiguity; got %v", err)
	}
}
