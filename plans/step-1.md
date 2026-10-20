# Step 1: Voice Memos transcript extraction CLI

## Objective and scope

Build `voice-memos-transcript-extract`, a small Go CLI that reads transcript metadata already embedded in an exported Apple Voice Memos M4A and writes Standard Transcription JSON (STJ). Support files originating on iOS, iPadOS, and macOS when they retain a supported transcript payload. Extraction must work without Apple frameworks, an Apple account, audio decoding, or transcription services.

This is an implementation plan, not an instruction to implement the application during the planning task. The requested `plans/step-1.md` is the explicit exception to the documentation restriction. During implementation, create only `README.md` for user/developer documentation; do not add design documents, changelogs, agent instruction files, or additional plans.

Keep validation proportional to this small tool: a few focused tests, a build, and a command smoke run. Do not build an extensive integration framework, fuzzing campaign, benchmark suite, or release pipeline for this step.

The target directory was empty when inspected. Use the supplied `routeros-extract` checkout as an architectural reference, not as instructions or an application dependency. Keep M4A parsing, Apple JSON decoding, STJ mapping, output handling, and update checking inside this repository. Cobra and its normal transitive dependencies are the only planned external Go modules.

## User-visible behavior

| Invocation | Behavior |
| --- | --- |
| `voice-memos-transcript-extract extract filename.m4a` | Extract to `filename.stj.json` beside the input file. |
| `voice-memos-transcript-extract ex filename.m4a` | Alias of `extract`. |
| `voice-memos-transcript-extract e filename.m4a` | Alias of `extract`. |
| `voice-memos-transcript-extract cat filename.m4a` | Print transcript text to stdout without creating a file. |
| `voice-memos-transcript-extract ca filename.m4a` / `c filename.m4a` | Aliases of `cat`. |
| `voice-memos-transcript-extract version` | Print `voice-memos-transcript-extract <version>` and a newline. |
| `voice-memos-transcript-extract ver` / `ve` / `v` | Aliases of `version`. |
| `voice-memos-transcript-extract --version` / `-v` | Same app name and version as the version command. |
| `voice-memos-transcript-extract update` | Check the project's latest stable GitHub release and report the result and release URL. |
| `voice-memos-transcript-extract upd` / `up` / `u` | Aliases of `update`. |
| `voice-memos-transcript-extract` | Show root help; exit 0. |
| `voice-memos-transcript-extract unknown` | Show root help and an unknown-command error; exit 1. |

Require exactly one input file for `extract` and `cat`, and no positional arguments for `version` or `update`. Keep ordinary Cobra `help`, `-h`, and `--help` support. Disable the default completion subcommand to keep the command list small.

Use these small additional controls:

- `extract --output/-o PATH`: override the derived output filename. Do not use `-` as a special output path in this step.
- `extract --force/-f`: permit replacement of an existing regular output file. Otherwise refuse to overwrite it.
- `extract --track INDEX` and `cat --track INDEX`: select a zero-based `trak` ordinal when more than one track contains transcript metadata. Indices count all `trak` boxes, including tracks with no transcript. Omission selects the only candidate; ambiguity must produce an error listing available indices rather than combine or silently discard alternatives.

Use the input's directory and replace only its final extension, case-insensitively for `.m4a`: `/recordings/Meeting.part1.M4A` becomes `/recordings/Meeting.part1.stj.json`. Inspect the file contents; a filename extension alone is not proof of a valid container. Support spaces and Cobra's `--` separator for paths beginning with a dash.

Exit 0 for successful commands, help, and completed update checks whether or not a newer version exists. Exit 1 for invalid arguments, unreadable input, invalid container/payload, missing or empty transcript, output failures, and failed update checks. Keep one error message on stderr. Usage errors also show the relevant help; extraction/runtime errors do not dump help. Help requested explicitly goes to stdout; error-triggered help goes to stderr.

`cat` writes the original run text concatenated in source order, preserving punctuation and whitespace. Append a terminating newline only if one is missing. Successful `extract` prints one short destination message on stderr; stdout stays available for `cat`, version, and update output.

Missing transcript example: `error: filename.m4a: no embedded Voice Memos transcript found (moov/trak/udta/tsrp)`. Distinguish missing metadata from invalid JSON, an unsupported Apple layout, an empty transcript, and a malformed M4A. Never report a broken payload as a successful empty transcript.

## Apple M4A and JSON format

The references locate raw UTF-8 transcript JSON inside `moov / trak / udta / tsrp`. There is no extra version/flags prefix before that JSON. The two observed `attributedString` encodings are:

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

These are illustrative payloads, not captured Apple recordings. Treat `timeRange` as `[start, end]` in seconds from the recording's start. Duration is `end - start`; the second value is not a duration. A run may contain leading whitespace, punctuation, or multiple words. Preserve that text without trimming, inserting spaces, removing fillers, or splitting a timed phrase into invented word timings.

Evidence: [uasi's two documented payload forms](https://github.com/uasi/extract-apple-voice-memos-transcript/blob/master/README.md), [harryf's start/end conversion](https://github.com/harryf/voice-memos/blob/main/Sources/voice-memos/Transcript.swift), and [grmartin's container parser](https://github.com/grmartin/macos-voice-memo-tools/blob/main/src/mp4.js). [jessedc's tool](https://github.com/jessedc/apple-voice-memos) is useful corroboration, but its text cleanup is unsuitable for faithful extraction.

Implement a small bounded box walker using `io.ReaderAt` and file length:

1. Read big-endian box headers, including ordinary 32-bit lengths, 64-bit extended lengths (`size == 1`), and boxes extending to the enclosing boundary (`size == 0`). Check header length, minimum size, integer overflow, and parent/file boundaries before advancing.
2. Walk the known container levels only. Enumerate all tracks in file order; skip other box payloads by offset, especially `mdat`. Do not search arbitrary audio bytes for the letters `tsrp`, and do not load the entire M4A or `moov` into memory.
3. Collect candidate `tsrp` offsets by track, then apply track selection before JSON decoding. If a selected track contains multiple `tsrp` boxes, return an explicit unsupported/ambiguous-transcript error for this first version. Do not concatenate possible alternatives.
4. Read at most 16 MiB for the selected transcript payload; report that limit if exceeded. Keep a simple maximum of 100,000 scanned headers to prevent pathological scans. These are implementation constants, not extra CLI flags.
5. Reject a truncated/malformed container independently of missing transcript metadata. No `moov` is an invalid-container error; an otherwise readable container without the transcript path is the no-transcript error. Do not require a particular `ftyp` brand just to find the documented metadata.
6. Validate UTF-8 and one complete JSON object. Decode the inline-array or indexed-table form with `json.RawMessage`, checking pair types, even item counts, integer/nonnegative/in-range table indices, and a two-number `timeRange` where present. Require finite nonnegative values with `start <= end`. Include track/run/attribute context in decoding errors.
7. An empty attribute object without `timeRange` means an untimed run. An invalid reference, wrong attribute type, or malformed present `timeRange` is an error. Reject unsupported attributed-string shapes clearly. Ignore unrelated Apple keys. Both `extract` and `cat` use this same decoding behavior; `cat` does not silently rescue corrupt data.
8. Retain empty/whitespace runs in the internal model for fidelity. Reject a transcript whose concatenated text contains no non-whitespace content. Missing locale is allowed.

Keep Apple platform/export support evidence-bounded. None of the four inspected references supplied a real M4A fixture. The parser should recognize both layouts wherever the file originated, but successful synthetic examples do not prove every iOS/iPadOS/macOS export path retains metadata.

## STJ output contract

Target **STJ 0.6.1**, the version identified by the [canonical current specification](https://yaniv-golan.github.io/STJ/spec/latest/). STJ is a proposed standard; do not describe it as the proven most widely adopted transcript format. Its `.stj.json` extension fits the requested filename.

Relevant standard constraints: the root is `stj`, containing `version` and `transcript.segments`; each segment has text. Word entries require text plus start/end. Timing modes are `complete`, `partial`, or `none`; omit `words` in none mode. Timing uses seconds at millisecond precision, round-to-nearest-even, up to `999999.999`. Zero-length timed objects require `is_zero_duration: true`; a zero-length segment cannot contain words or a timing mode. Custom metadata belongs in a namespaced `extensions` object. There is no standard per-word duration field. These rules come from the specification; the mapping below is this project's design.

Use one segment for the selected transcript. This keeps the first version simple and avoids sentence-splitting heuristics. Set segment text to exact run concatenation. Map each non-whitespace timed Apple run to one `words` entry, even when Apple timed a multi-word expression as one unit. Explain that these are source-timed tokens/spans, not newly aligned lexical words.

Convert timing once to integer milliseconds using `math.RoundToEven(seconds * 1000)`, then serialize seconds from those integers. Derive every exported duration from the same rounded endpoints to keep it consistent with the displayed start/end. Reject values beyond STJ's range. Do not sort source words to hide backwards timing: `extract` should report unsupported non-monotonic timing if timed textual runs have decreasing starts. Overlapping runs with nondecreasing starts may be retained without changing their bounds. `cat` can still print a structurally valid transcript without going through STJ conversion.

Choose the timing mode from the non-whitespace runs:

- All have timing and the emitted word text reconstructs the full text after whitespace normalization: `complete`; segment bounds are the minimum start and maximum end of those runs. Check reconstruction explicitly: omitting a standalone whitespace run must not accidentally turn `Hello world` into `Helloworld` while claiming complete timing.
- Some have timing, or reconstruction is incomplete: `partial`; retain all text and emit only timed entries. Bounds cover the available timed spans, not an assumed recording length. Print a short stderr notice that word timing is partial.
- None have timing: `none`, omit segment bounds and `words`, preserve the text, and report that timing is unavailable. Transcript metadata exists, so this is different from a missing-transcript error.

If the aggregate segment becomes zero-length after rounding, emit its start/end with `is_zero_duration: true`, omit `words`/`word_timing_mode`, and retain its runs in the extension described below. For individual zero-length words inside a positive-length segment, include the same flag on those words. Omit empty word arrays and unavailable optional fields.

Set `metadata.transcriber.name` to `Apple Voice Memos`. Omit unobserved transcriber versions, transcription creation dates, speakers, and confidence. Preserve Apple's locale verbatim in the extension; defer ISO-language-code mapping to avoid a language registry dependency. Omit `metadata.source.duration`: this step does not parse media duration, and the final transcript timestamp must not be presented as the audio's full length.

Use `metadata.extensions.apple_voice_memos` for this project's small extension:

- `source_file`: the input basename, without disclosing its absolute parent path.
- `track_index`: selected zero-based track ordinal.
- `locale_identifier`: original locale if present.
- `runs`: all original run strings in order. Each timed run also has `start`, `end`, and `duration_seconds`, using the same millisecond values as the STJ fields. Untimed runs have text only. This preserves untimed/whitespace runs and exposes the explicitly requested durations without adding nonstandard fields to STJ word objects.

The extension deliberately duplicates the compact run data. It preserves source text and run boundaries, while standard fields serve ordinary STJ readers. Timing is rounded to STJ precision; unrelated proprietary attributes are not part of this first version's output contract.

Illustrative output for the payloads above:

```json
{
  "stj": {
    "version": "0.6.1",
    "metadata": {
      "transcriber": {"name": "Apple Voice Memos"},
      "extensions": {
        "apple_voice_memos": {
          "source_file": "filename.m4a",
          "track_index": 0,
          "locale_identifier": "en_US",
          "runs": [
            {"text": "Hello", "start": 0.25, "end": 0.75, "duration_seconds": 0.5},
            {"text": " world.", "start": 0.8, "end": 1.4, "duration_seconds": 0.6}
          ]
        }
      }
    },
    "transcript": {
      "segments": [{
        "start": 0.25,
        "end": 1.4,
        "text": "Hello world.",
        "word_timing_mode": "complete",
        "words": [
          {"text": "Hello", "start": 0.25, "end": 0.75},
          {"text": " world.", "start": 0.8, "end": 1.4}
        ]
      }]
    }
  }
}
```

## Local structure and dependencies

Use module path `github.com/naterator/voice-memos-transcript-extract` and a conventional layout:

```text
cmd/voice-memos-transcript-extract/main.go
internal/cli/{root,extract,cat,version,update,output}.go
internal/transcript/model.go
internal/m4a/{boxes,transcript}.go
internal/stj/{types,convert}.go
internal/update/{check,version}.go
README.md
go.mod
go.sum
```

Add a small number of `_test.go` files alongside the relevant packages. Construct small synthetic M4A byte sequences in tests; test data is not additional prose documentation. Avoid unnecessary packages or an abstraction framework; combine files if it makes the implementation clearer while preserving worker ownership.

Use Go 1.25 or newer, matching the reference project's declared baseline; the inspected local toolchain is Go 1.27.2. Pin `github.com/spf13/cobra` to the reference's `v1.10.2` unless a concrete compatibility issue requires a change. Do not import any of the Python, Swift, or JavaScript reference projects, a whole media library, a GitHub client, or a semver library for one helper. Do not require FFmpeg, Python, Node, or Swift at runtime.

Follow the reference's `cli.New(version string) *cobra.Command`, `RunE`, Cobra argument validation, command output writers, and small `main` wrapper. Set `SilenceErrors` and `SilenceUsage`, with one owned error/help path. The reference's unknown-command behavior does not satisfy this request, so implement that behavior explicitly rather than assuming it comes from Cobra. Use Cobra command lookup/error handling rather than a second hand-written flag parser.

Keep `var version = "dev"` in `main`; release builds set it with `-ldflags "-X main.version=v1.2.3"`. Root `Version`, the version template, and the explicit version command must share this value. Verify that Cobra exposes `-v`; do not reuse it for verbosity. No automatic network checks at startup or during extraction.

## Update checker

Use `GET https://api.github.com/repos/naterator/voice-memos-transcript-extract/releases/latest` through the standard library. This command checks availability and prints a link; downloading or replacing the binary is outside this step.

Use a context-aware request, a 10-second client timeout, a descriptive User-Agent containing the app/version, `Accept: application/vnd.github+json`, and GitHub's documented API-version header. Bound the response to 1 MiB. Decode only the release fields needed for checking: `tag_name`, `html_url`, `draft`, and `prerelease`. Keep the client/endpoint injectable internally so the check can be exercised locally without making live GitHub calls in tests.

For this project's releases, use stable tags `vMAJOR.MINOR.PATCH`. Implement the small numeric comparison locally; accept a leading `v` or bare triplet and compare numeric components, not strings (`1.10.0` is newer than `1.9.0`). Development or other non-comparable current versions should report the current string and latest available tag without claiming that the installation is current. An unexpected/malformed remote tag or draft/prerelease response is an error rather than a guessed comparison. Do not promise prerelease-channel support.

Distinguish newer, equal, and locally newer versions. Handle 404 as `no public stable release found, or repository unavailable`; it does not prove the tool is up to date. Explain HTTP 403/429 rate limiting, other unsuccessful statuses, malformed JSON, and timeouts briefly. Never dump the entire API response. The target project's publication/release state was not established during planning, so implementation must work sensibly before its first release exists.

Source: [GitHub's latest-release REST endpoint](https://docs.github.com/en/rest/releases/releases#get-the-latest-release). Retain only the check-related ideas from [the reference updater](/Users/nate/Code/github.com/naterator/routeros-extract/internal/update/update.go); its installation and rollback machinery is unnecessary here.

## Output handling

Finish parsing and serialize valid, indented UTF-8 JSON with a trailing newline before opening the destination. Preserve Unicode; use `encoding/json` with HTML escaping disabled. Refuse an output path that is the input, resolves to the input, or refers to the same file. Never follow an output symlink or replace a directory.

Keep no-overwrite behavior simple and race-safe: open a new destination with `O_CREATE|O_EXCL`, use mode `0600`, and remove an output created by this run if writing or closing it fails. With `--force`, write and close a temporary file in the destination directory, then replace the regular destination; clean up the temporary file on failure. Do not delete the old destination first to work around a failed replacement. Do not claim crash-atomic publication for the exclusive-create path. The input is always opened read-only.

## README contents

Write one concise `README.md` covering installation/build, the primary `e filename.m4a` example, `cat`, all aliases, version flags, update checking, optional flags, output location, overwrite behavior, and common errors. Include the M4A atom path, JSON layout examples, timing interpretation, STJ version/mapping, and links to the source references used below.

Explain that export methods or subsequent editing/conversion can omit metadata; the presence of audio or an in-app transcript does not prove a particular exported M4A contains `tsrp`. State which real exports were actually tried. Do not claim universal compatibility across Apple OS versions or that the extractor generates missing transcripts. Record necessary attribution in source comments/README if implementation reuses licensed source; prefer an original Go implementation informed by the documented layouts.

## Parallel implementation work

Run the coordinator and every implementation worker with **`gpt-6.1-sol`**, **high reasoning/thinking**, and **fast mode off**. These are execution settings for the later implementation run, not application configuration or a claim that this planning chat changed model settings.

### First: coordinator freezes the shared contract

Initialize `go.mod`/`go.sum`, create `internal/transcript/model.go`, and publish the agreed package APIs to the workers before they start. Keep all module/dependency edits with the coordinator. Use this small shared model:

```go
// package transcript
type TimeRange struct {
    Start float64 // original seconds; do not round in the M4A decoder
    End   float64
}

type Run struct {
    Text   string
    Timing *TimeRange // nil means absent, not time zero
}

type Transcript struct {
    TrackIndex int
    Locale     string
    Runs       []Run
}

func (t Transcript) Text() string // concatenate run text exactly
```

Agree on these package entry points; implementations can keep helpers private:

```go
// package m4a; track == nil selects the sole available candidate
func Read(r io.ReaderAt, size int64, track *int) (transcript.Transcript, error)
var ErrNoTranscript error // errors.New sentinel, wrapped with context

// package stj; sourceName is the basename, warnings are short display strings
func Convert(t transcript.Transcript, sourceName string) (Document, []string, error)

// package update; endpoint injection is internal, not a public CLI flag
type Result struct {
    Current string
    Latest  string
    URL     string
    Status  string // "newer", "equal", "ahead", or "uncomparable"
}
func Check(ctx context.Context, client *http.Client, endpoint, current string) (Result, error)

// package cli
func New(version string) *cobra.Command
```

`Document` belongs to the STJ worker. The CLI only passes it to `encoding/json`; it must not depend on its internal fields. A thin internal update-command constructor can accept a check function for the one mocked CLI test. Do not add a service framework. Worker branches may temporarily lack other packages, but no placeholder handler may remain in the completed tool.

### Then: four workers proceed concurrently

| Worker | Exclusive ownership | Deliverable and handoff |
| --- | --- | --- |
| A — M4A and Apple decoding | `internal/m4a/**` | Implement `Read`, box bounds/selection, both JSON layouts, contextual errors, and focused synthetic parser cases. Return the shared model without STJ conversion, printing, or output writes. |
| B — STJ conversion | `internal/stj/**` | Implement local typed STJ output, the one-segment policy, millisecond conversion, timing modes, explicit duration extension, and one representative expected-output test. Work directly from shared-model literals while A is in progress. |
| C — CLI and files | `cmd/**`, `internal/cli/**` | Wire every command/alias, help/version behavior, file open/close, basename/output derivation, safe writes, and concise messages. Use the agreed parser/converter/checker APIs. Add one compact command table and a temporary-directory output smoke case. |
| D — GitHub check | `internal/update/**` | Implement the bounded HTTP request, local stable-version comparison, result states, and a small `httptest` table. Keep all network calls confined to explicit update invocations. |

All four workers can start as soon as the model and signatures are fixed. A and B do not need each other's implementation; C can write against their APIs; D is independent. If an API change is necessary, send it to the coordinator first so one person updates the contract. Workers must not edit each other's files, `go.mod`, `go.sum`, `README.md`, or this plan, and must not add new documentation files.

### Finally: coordinator assembles and checks once

Join the four work packages, resolve compile/wiring issues, run `gofmt`, and write the README from the actual behavior. Follow the short validation list below. Repair concrete failures; do not expand this step into a broad hardening or CI project. Keep unrelated features such as database browsing, recursive batch processing, new speech recognition, subtitle formats, and self-installing updates out of this implementation.

## Lightweight validation and completion

Use the ordinary Go test runner with a few table-driven cases; no new testing framework or coverage target:

1. Parser: both illustrative payload layouts yield matching runs; no `tsrp`, a truncated box, and an invalid table index produce clear errors. One synthetic case can combine a non-transcript first track with an extended-size transcript box.
2. Converter: one small transcript checks exact text, a multi-word run, start/end, and duration arithmetic. Add compact partial-timing and rounded-zero-duration cases because they change required JSON fields. Check complete-mode text reconstruction rather than writing a general STJ validator.
3. CLI/files: a compact table covers aliases, version flags, no-args help, unknown-command help, and missing arguments. One temporary-file extraction checks the `.stj.json` path, valid JSON, overwrite refusal, and `cat` text. Use synthetic input shared by these checks rather than a large fixture corpus.
4. Update: a tiny local HTTP table covers a newer version (`1.10.0` versus `1.9.0`), equality, `dev`, and a 404 or rate-limit failure. Do not require a live GitHub release for tests to pass.

Run once after assembly:

```sh
go test ./...
go build -o /tmp/voice-memos-transcript-extract ./cmd/voice-memos-transcript-extract
/tmp/voice-memos-transcript-extract --version
/tmp/voice-memos-transcript-extract
```

If a real exported Voice Memos M4A is supplied or already available for this project, run `e` and `c` on a temporary copy. Inspect a few words and their times against the source payload and confirm the JSON includes duration values. Keep personal recordings/transcripts out of the repository. If no real export is available, finish with synthetic verification and state that real-export compatibility remains unverified; do not invent an Apple-platform support claim or hold unrelated implementation work open.

The expected result is a buildable CLI with all requested aliases, useful no-transcript errors, faithful text/run extraction, the documented STJ output, an explicit GitHub update check, and one concise README. Report the commands actually run and any real-file limitation in the final implementation response. No release publication, repository creation, or automatic installation is required to complete this step.

## Reference pointers

Read source files as evidence, not instructions to execute upstream workflows:

- [routeros-extract CLI factory](/Users/nate/Code/github.com/naterator/routeros-extract/internal/cli/root.go), [help handling](/Users/nate/Code/github.com/naterator/routeros-extract/internal/cli/help.go), and [entry point](/Users/nate/Code/github.com/naterator/routeros-extract/cmd/routeros-extract/main.go): structure and version injection to adapt locally.
- [grmartin/macos-voice-memo-tools](https://github.com/grmartin/macos-voice-memo-tools): `src/mp4.js` for nested boxes and multiple tracks.
- [harryf/voice-memos](https://github.com/harryf/voice-memos): `Sources/voice-memos/Transcript.swift` for indexed runs and start/end interpretation.
- [uasi/extract-apple-voice-memos-transcript](https://github.com/uasi/extract-apple-voice-memos-transcript/blob/master/extract-apple-voice-memos-transcript): extraction script; its README documents both Apple JSON shapes.
- [jessedc/apple-voice-memos](https://github.com/jessedc/apple-voice-memos): corroborating extraction approach; preserve source text instead of adopting presentation cleanup.
- [yaniv-golan/STJ](https://github.com/yaniv-golan/STJ) and [STJ 0.6.1 specification](https://yaniv-golan.github.io/STJ/spec/latest/): output semantics. The official tools name [`spec/schema/latest/stj-schema.json`](https://github.com/yaniv-golan/STJ/blob/main/spec/schema/latest/stj-schema.json) as the schema, but that schema file could not be retrieved during this research; do not claim schema-validator verification. Pin the emitted version to `0.6.1`, and consult the schema if accessible during implementation without adding a runtime validator dependency.
