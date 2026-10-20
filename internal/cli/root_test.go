package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naterator/voice-memos-transcript-extract/internal/update"
	"github.com/spf13/cobra"
)

func execute(root *cobra.Command, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestCommands(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"ver"}, {"ve"}, {"v"}, {"--version"}, {"-v"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, stderr, err := execute(New("v1.2.3"), args...)
			if err != nil || out != appName+" v1.2.3\n" || stderr != "" {
				t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
			}
		})
	}
	for _, alias := range []string{"extract", "ex", "e", "cat", "ca", "c", "update", "upd", "up", "u"} {
		t.Run(alias+" help", func(t *testing.T) {
			out, stderr, err := execute(New("dev"), alias, "--help")
			if err != nil || !strings.Contains(out, "Usage:") || stderr != "" {
				t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		args      []string
		wantError bool
	}{
		{"no args", nil, false}, {"help", []string{"help"}, false},
		{"unknown", []string{"unknown"}, true}, {"missing file", []string{"e"}, true},
		{"extra version arg", []string{"version", "extra"}, true},
		{"invalid flag", []string{"cat", "--bad-flag"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, stderr, err := execute(New("dev"), tc.args...)
			if (err != nil) != tc.wantError {
				t.Fatalf("err=%v", err)
			}
			if tc.wantError {
				if out != "" || !strings.Contains(stderr, "Usage:") {
					t.Fatalf("out=%q stderr=%q", out, stderr)
				}
			} else if !strings.Contains(out, "Usage:") || stderr != "" || strings.Contains(out, "completion") {
				t.Fatalf("out=%q stderr=%q", out, stderr)
			}
		})
	}
}

func TestExtractionFilesAndCat(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "Meeting.part1.M4A")
	payload := []byte(`{"attributedString":["Hello",{"timeRange":[0.25,0.75]}," 世界 & <all>.",{"timeRange":[0.8,1.4]}],"locale":{"identifier":"en_US"}}`)
	if err := os.WriteFile(input, testBox("moov", testBox("trak", testBox("udta", testBox("tsrp", payload)))), 0600); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := execute(New("dev"), "e", input)
	if err != nil || out != "" || !strings.Contains(stderr, "Wrote ") {
		t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
	}
	destination := filepath.Join(directory, "Meeting.part1.stj.json")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) || !bytes.HasSuffix(data, []byte("\n")) || !bytes.Contains(data, []byte("世界 & <all>")) {
		t.Fatalf("unexpected JSON: %s", data)
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("info=%v err=%v", info, err)
	}
	_, stderr, err = execute(New("dev"), "ex", input)
	if err == nil || !strings.Contains(err.Error(), "already exists") || strings.Contains(stderr, "Usage:") {
		t.Fatalf("stderr=%q err=%v", stderr, err)
	}
	if _, _, err := execute(New("dev"), "extract", "--force", input); err != nil {
		t.Fatal(err)
	}
	out, stderr, err = execute(New("dev"), "c", "--", input)
	if err != nil || out != "Hello 世界 & <all>.\n" || stderr != "" {
		t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
	}
	for _, target := range []string{input, filepath.Join(directory, "input-link")} {
		if target != input {
			if err := os.Symlink(input, target); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := execute(New("dev"), "e", input, "-f", "-o", target); err == nil {
			t.Fatalf("accepted unsafe output %q", target)
		}
	}
}

func TestUpdateCommand(t *testing.T) {
	root := New("dev")
	for _, cmd := range root.Commands() {
		if cmd.Name() == "update" {
			root.RemoveCommand(cmd)
		}
	}
	root.AddCommand(newUpdateCommand("dev", func(_ context.Context, client *http.Client, endpoint, current string) (update.Result, error) {
		if client.Timeout <= 0 || !strings.HasSuffix(endpoint, "/releases/latest") || current != "dev" {
			t.Fatal("incorrect update request")
		}
		return update.Result{Current: current, Latest: "v1.10.0", URL: "https://example.com/release", Status: "uncomparable"}, nil
	}))
	out, stderr, err := execute(root, "u")
	if err != nil || stderr != "" || !strings.Contains(out, "dev; latest release: v1.10.0") || !strings.Contains(out, "https://example.com/release") {
		t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
	}
}

func testBox(kind string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box, uint32(len(box)))
	copy(box[4:8], kind)
	copy(box[8:], payload)
	return box
}
