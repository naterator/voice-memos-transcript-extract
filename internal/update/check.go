// Package update checks GitHub releases without downloading or replacing files.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultEndpoint = "https://api.github.com/repos/naterator/voice-memos-transcript-extract/releases/latest"
	checkTimeout    = 10 * time.Second
	maxResponseSize = 1 << 20
)

type Result struct {
	Current string
	Latest  string
	URL     string
	Status  string // newer, equal, ahead, or uncomparable
}

// Check retrieves the latest stable release. A nil client and empty endpoint use
// the standard HTTP client and this project's GitHub endpoint. The supplied
// client is copied so enforcing the timeout does not change its configuration.
func Check(ctx context.Context, client *http.Client, endpoint, current string) (Result, error) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if client == nil {
		client = http.DefaultClient
	}
	boundedClient := *client
	if boundedClient.Timeout <= 0 || boundedClient.Timeout > checkTimeout {
		boundedClient.Timeout = checkTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create update request: %w", err)
	}
	req.Header.Set("User-Agent", "voice-memos-transcript-extract/"+current)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	resp, err := boundedClient.Do(req)
	if err != nil {
		return Result{}, requestError(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Result{}, errors.New("no public stable release found, or repository unavailable")
	case http.StatusForbidden:
		return Result{}, errors.New("GitHub update check failed (HTTP 403): access denied or API rate limit reached; try again later")
	case http.StatusTooManyRequests:
		return Result{}, errors.New("GitHub API rate limit reached (HTTP 429); try again later")
	default:
		return Result{}, fmt.Errorf("GitHub update check failed (HTTP %d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return Result{}, requestError(err)
	}
	if len(body) > maxResponseSize {
		return Result{}, errors.New("GitHub release response exceeds 1 MiB limit")
	}
	var release struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&release); err != nil {
		return Result{}, errors.New("invalid GitHub release JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Result{}, errors.New("invalid GitHub release JSON: expected one object")
	}
	if release.Draft || release.Prerelease {
		return Result{}, errors.New("GitHub latest release is a draft or prerelease; expected a stable release")
	}
	latestVersion, ok := parseVersion(release.Tag)
	if !ok {
		return Result{}, errors.New("GitHub release tag is not a stable MAJOR.MINOR.PATCH version")
	}
	releaseURL, err := url.Parse(release.URL)
	if err != nil || releaseURL.Host == "" || (releaseURL.Scheme != "https" && releaseURL.Scheme != "http") {
		return Result{}, errors.New("GitHub release response has no valid release URL")
	}
	result := Result{Current: current, Latest: release.Tag, URL: release.URL, Status: "uncomparable"}
	if currentVersion, ok := parseVersion(current); ok {
		switch compareVersions(currentVersion, latestVersion) {
		case -1:
			result.Status = "newer"
		case 0:
			result.Status = "equal"
		case 1:
			result.Status = "ahead"
		}
	}
	return result, nil
}

func requestError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("GitHub update check timed out: %w", err)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("GitHub update check canceled: %w", err)
	}
	return fmt.Errorf("GitHub update check failed: %w", err)
}
