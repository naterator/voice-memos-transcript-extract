// Package m4a reads transcript metadata embedded in Apple Voice Memos exports.
package m4a

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/naterator/voice-memos-transcript-extract/internal/transcript"
)

const (
	maxHeaders = 100_000
	maxPayload = 16 << 20
)

// ErrNoTranscript means that the container has no supported transcript path.
var ErrNoTranscript = errors.New("no embedded Voice Memos transcript found (moov/trak/udta/tsrp)")

type box struct {
	kind       string
	start, end int64 // payload boundaries, excluding the box header
}

type walker struct {
	r       io.ReaderAt
	headers int
}

// walk validates each immediate box before visiting its payload. Bounds checks
// use the remaining parent length before converting any unsigned box size.
func (w *walker) walk(start, end int64, path string, visit func(box) error) error {
	for offset := start; offset < end; {
		w.headers++
		if w.headers > maxHeaders {
			return fmt.Errorf("malformed M4A: scanned header limit (%d) exceeded", maxHeaders)
		}
		var header [16]byte
		if end-offset < 8 {
			return fmt.Errorf("malformed M4A: %s at offset %d: truncated box header", path, offset)
		}
		if err := readAt(w.r, header[:8], offset); err != nil {
			return fmt.Errorf("malformed M4A: %s at offset %d: read box header: %w", path, offset, err)
		}
		kind := string(header[4:8])
		length := uint64(binary.BigEndian.Uint32(header[:4]))
		headerSize := int64(8)
		switch length {
		case 0:
			length = uint64(end - offset)
		case 1:
			headerSize = 16
			if end-offset < headerSize {
				return fmt.Errorf("malformed M4A: %s/%q at offset %d: truncated extended box header", path, kind, offset)
			}
			if err := readAt(w.r, header[8:], offset+8); err != nil {
				return fmt.Errorf("malformed M4A: %s/%q at offset %d: read extended box header: %w", path, kind, offset, err)
			}
			length = binary.BigEndian.Uint64(header[8:])
		}
		if length < uint64(headerSize) {
			return fmt.Errorf("malformed M4A: %s/%q at offset %d: box size %d is smaller than its header", path, kind, offset, length)
		}
		if length > uint64(end-offset) {
			return fmt.Errorf("malformed M4A: %s/%q at offset %d: box exceeds enclosing boundary", path, kind, offset)
		}
		next := offset + int64(length)
		if err := visit(box{kind: kind, start: offset + headerSize, end: next}); err != nil {
			return err
		}
		offset = next
	}
	return nil
}

func readAt(r io.ReaderAt, p []byte, offset int64) error {
	n, err := r.ReadAt(p, offset)
	if n == len(p) {
		return nil
	}
	if err == nil {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// Read extracts a sole transcript candidate, or the requested zero-based track.
// It only reads box headers and the selected transcript's JSON payload.
func Read(r io.ReaderAt, size int64, track *int) (transcript.Transcript, error) {
	var empty transcript.Transcript
	if size < 0 {
		return empty, fmt.Errorf("malformed M4A: negative file size")
	}
	if track != nil && *track < 0 {
		return empty, fmt.Errorf("track index must be nonnegative")
	}
	w := walker{r: r}
	var tracks [][]box
	foundMoov := false
	err := w.walk(0, size, "file", func(top box) error {
		if top.kind != "moov" {
			return nil
		}
		foundMoov = true
		return w.walk(top.start, top.end, "moov", func(child box) error {
			if child.kind != "trak" {
				return nil
			}
			index := len(tracks)
			tracks = append(tracks, nil)
			path := fmt.Sprintf("moov/trak[%d]", index)
			return w.walk(child.start, child.end, path, func(child box) error {
				if child.kind != "udta" {
					return nil
				}
				return w.walk(child.start, child.end, path+"/udta", func(child box) error {
					if child.kind == "tsrp" {
						tracks[index] = append(tracks[index], child)
					}
					return nil
				})
			})
		})
	})
	if err != nil {
		return empty, err
	}
	if !foundMoov {
		return empty, fmt.Errorf("malformed M4A: no moov box found")
	}
	selected := -1
	if track != nil {
		selected = *track
		if selected >= len(tracks) {
			return empty, fmt.Errorf("track index %d is out of range (%d tracks)", selected, len(tracks))
		}
		if len(tracks[selected]) == 0 {
			return empty, fmt.Errorf("track %d: %w", selected, ErrNoTranscript)
		}
	} else {
		var available []string
		for i, payloads := range tracks {
			if len(payloads) > 0 {
				selected = i
				available = append(available, fmt.Sprint(i))
			}
		}
		switch len(available) {
		case 0:
			return empty, fmt.Errorf("M4A: %w", ErrNoTranscript)
		case 1:
		default:
			return empty, fmt.Errorf("ambiguous transcript: available track indices: %s; select one with --track", strings.Join(available, ", "))
		}
	}
	if len(tracks[selected]) != 1 {
		return empty, fmt.Errorf("track %d: unsupported ambiguous transcript: found %d tsrp boxes", selected, len(tracks[selected]))
	}
	payload := tracks[selected][0]
	if payload.end-payload.start > maxPayload {
		return empty, fmt.Errorf("track %d: transcript payload exceeds the 16 MiB limit", selected)
	}
	data := make([]byte, int(payload.end-payload.start))
	if err := readAt(r, data, payload.start); err != nil {
		return empty, fmt.Errorf("track %d: read transcript payload: %w", selected, err)
	}
	t, err := decode(data)
	if err != nil {
		return empty, fmt.Errorf("track %d: %w", selected, err)
	}
	t.TrackIndex = selected
	return t, nil
}
