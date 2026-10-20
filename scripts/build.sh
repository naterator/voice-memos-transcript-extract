#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:-dist}"
version="${VERSION:-dev}"
mkdir -p "$output_dir"

# Ask the selected Go toolchain for its actual OS/architecture combinations.
# Mobile targets require platform SDKs and are outside this desktop/server CLI.
targets="$(go tool dist list)"
while IFS=/ read -r target_os target_arch; do
  case "$target_arch" in
    amd64|arm64) ;;
    *) continue ;;
  esac
  case "$target_os" in
    android|ios) continue ;;
  esac

  asset="voice-memos-transcript-extract-$target_os-$target_arch"
  if [ "$target_os" = windows ]; then
    asset="$asset.exe"
  fi
  printf 'Building %s/%s\n' "$target_os" "$target_arch"
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
    go build -trimpath -ldflags "-X main.version=$version" \
      -o "$output_dir/$asset" ./cmd/voice-memos-transcript-extract
done <<< "$targets"
