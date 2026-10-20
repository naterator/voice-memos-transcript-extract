# voice-memos-transcript-extract

A small Go CLI that extracts transcript metadata already embedded in Apple Voice Memos M4A files. It writes [Standard Transcription JSON (STJ) 0.6.1](https://yaniv-golan.github.io/STJ/spec/latest/) or prints the transcript text. It works on exported files without Apple frameworks or a transcription service.

## Build and use

Requires Go 1.25 or newer:

```sh
go build -o voice-memos-transcript-extract ./cmd/voice-memos-transcript-extract
./voice-memos-transcript-extract e filename.m4a
```

This writes `filename.stj.json` beside the input file. Paths containing spaces should be quoted. Only the final filename extension is replaced, so `Meeting.part1.M4A` becomes `Meeting.part1.stj.json`.

```sh
# Print the original transcript text.
./voice-memos-transcript-extract c "Meeting notes.m4a"

# Choose an output path, or explicitly replace an existing output file.
./voice-memos-transcript-extract e filename.m4a -o transcript.stj.json
./voice-memos-transcript-extract e filename.m4a --force

# Select a transcript track when the file contains more than one.
./voice-memos-transcript-extract e filename.m4a --track 1

# Use -- before a filename beginning with a dash.
./voice-memos-transcript-extract e -- -recording.m4a

./voice-memos-transcript-extract --version
./voice-memos-transcript-extract update
```

| Command | Aliases | Purpose |
| --- | --- | --- |
| `extract FILE` | `ex`, `e` | Write a `.stj.json` transcript. |
| `cat FILE` | `ca`, `c` | Print transcript text without creating a file. |
| `version` | `ver`, `ve`, `v` | Print the application name and version. |
| `update` | `upd`, `up`, `u` | Check the latest stable GitHub release. |

`--version` and `-v` also print the version. Run without a command for help, or use `--help` on any command. An unknown command prints help and exits with an error.

`extract` accepts `--output`/`-o` and `--force`/`-f`. It refuses an existing output unless `--force` is supplied, and never overwrites the input or follows an output symlink. New transcript files use owner-only permissions (`0600`). Both `extract` and `cat` accept `--track INDEX`; this is the zero-based position among all M4A tracks, including tracks without transcripts. With multiple transcript tracks, the tool asks you to choose rather than silently selecting one.

`cat` preserves the original punctuation and whitespace, adding a final newline if needed. Extraction status and timing notices go to stderr. Commands exit 0 on success and 1 on errors, including missing transcript metadata.

The update command checks [this project's GitHub releases](https://github.com/naterator/voice-memos-transcript-extract/releases) and reports a link. It does not install anything. Stable release tags use `vMAJOR.MINOR.PATCH`; a development build reports the latest version without claiming to be current. No other command makes network requests. To embed a release version when building:

```sh
go build -ldflags "-X main.version=v1.2.3" -o voice-memos-transcript-extract ./cmd/voice-memos-transcript-extract
```

CI runs formatting, vet, race-enabled tests, and cross-builds on Ubuntu. Publishing a stable GitHub release builds and attaches binaries for every desktop/server `amd64` and `arm64` combination reported by the selected Go toolchain. All builds use the same Ubuntu runner platform; Android and iOS are excluded because they need mobile SDKs. Windows binaries have an `.exe` extension.

To run the same cross-build locally from the project directory:

```sh
VERSION=v1.2.3 bash scripts/build.sh dist
```

`VERSION` defaults to `dev`; the output directory defaults to `dist`. The workflows smoke-test the Ubuntu binary; the other binaries are cross-compiled without execution.

## M4A transcript metadata

The observed Voice Memos format stores raw UTF-8 JSON in the proprietary `tsrp` box at:

```text
moov / trak / udta / tsrp
```

There is no extra version/flags prefix before the JSON. The parser walks container headers and reads only the selected transcript payload; it skips the audio data. It supports ordinary, extended-size, and enclosing-boundary boxes. A selected transcript is limited to 16 MiB, and container scanning to 100,000 headers.

Two attributed-string layouts are supported. These examples are illustrative:

```json
{
  "attributedString": [
    "Hello", {"timeRange": [0.25, 0.75]},
    " world.", {"timeRange": [0.8, 1.4]}
  ],
  "locale": {"identifier": "en_US"}
}
```

```json
{
  "attributedString": {
    "runs": ["Hello", 0, " world.", 1],
    "attributeTable": [
      {"timeRange": [0.25, 0.75]},
      {"timeRange": [0.8, 1.4]}
    ]
  },
  "locale": {"identifier": "en_US"}
}
```

`timeRange` contains **start and end**, in seconds relative to the recording's start. Duration is `end - start`. Run strings may include leading spaces or several words timed as one span; the tool preserves them without cleaning or re-transcribing the text. An attribute object without `timeRange` is an untimed run. Invalid JSON, malformed ranges, or invalid attribute references produce errors.

## JSON output

STJ is a proposed transcript interchange standard. Output uses the `stj` root with `version: "0.6.1"` and one `transcript.segments` entry containing the exact reconstructed text. Standard `words` entries retain Apple's timed spans, including multi-word spans. The tool does not invent timings for words Apple timed together.

Timing is rounded to milliseconds with round-to-nearest-even. `word_timing_mode` is `complete`, `partial`, or `none`, according to the available timing and text coverage. Untimed text remains in the segment. Zero-duration timing uses STJ's `is_zero_duration` flag; a zero-duration segment omits word-level timing. Backwards word starts are rejected during STJ conversion; `cat` can still print a structurally valid transcript.

`metadata.extensions.apple_voice_memos` contains the input basename, selected track index, original locale when present, and all original runs. Timed extension runs include `start`, `end`, and **`duration_seconds`**, calculated from the same rounded endpoints as the standard fields. Untimed runs contain text only. Unrelated proprietary attributes are not exported. The tool omits guessed speakers, confidence, language codes, and full audio duration.

For the payloads above, the segment text is `Hello world.`, its bounds are `0.25`–`1.4` seconds, and its two timed runs have durations `0.5` and `0.6` seconds. The resulting JSON is indented UTF-8 with a final newline.

## Missing metadata and compatibility

If the transcript box is absent, the command reports `no embedded Voice Memos transcript found (moov/trak/udta/tsrp)` and creates no output. Invalid containers, unsupported layouts, empty transcripts, and malformed metadata have separate errors. A transcript with valid text but no timings can still be exported with a timing notice.

The supported layouts may occur in files originating on iOS, iPadOS, or macOS. Export methods and subsequent conversions can remove metadata: an in-app transcript does not guarantee a particular exported M4A contains it. No real Apple export was supplied for this implementation; verification uses synthetic containers representing the documented layouts. Real-export compatibility remains unverified.

Format references: [grmartin/macos-voice-memo-tools](https://github.com/grmartin/macos-voice-memo-tools), [harryf/voice-memos](https://github.com/harryf/voice-memos), [uasi/extract-apple-voice-memos-transcript](https://github.com/uasi/extract-apple-voice-memos-transcript), and [jessedc/apple-voice-memos](https://github.com/jessedc/apple-voice-memos). CLI structure follows the local `routeros-extract` project. The parsing and conversion logic is implemented locally in Go; Cobra is the only direct external module.
