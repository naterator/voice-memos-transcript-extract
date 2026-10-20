package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	const releaseURL = "https://github.com/naterator/voice-memos-transcript-extract/releases/tag/v1.10.0"
	stable := fmt.Sprintf(`{"tag_name":"v1.10.0","html_url":%q,"draft":false,"prerelease":false}`, releaseURL)
	for _, tc := range []struct {
		name, current, body, wantStatus, wantError string
		code                                       int
	}{
		{name: "numeric newer", current: "v1.9.0", body: stable, wantStatus: "newer"},
		{name: "equal bare triplet", current: "1.10.0", body: stable, wantStatus: "equal"},
		{name: "locally ahead", current: "v2.0.0", body: stable, wantStatus: "ahead"},
		{name: "development", current: "dev", body: stable, wantStatus: "uncomparable"},
		{name: "not found", code: 404, wantError: "no public stable release"},
		{name: "rate limit", code: 429, wantError: "rate limit"},
		{name: "invalid remote tag", body: `{"tag_name":"v1.10.0-rc1"}`, wantError: "not a stable"},
		{name: "prerelease", body: `{"prerelease":true}`, wantError: "draft or prerelease"},
		{name: "malformed", body: `{`, wantError: "invalid GitHub release JSON"},
		{name: "trailing JSON", body: stable + `{}`, wantError: "expected one object"},
		{name: "oversized", body: strings.Repeat(" ", maxResponseSize+1), wantError: "1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" || r.Header.Get("User-Agent") != "voice-memos-transcript-extract/"+tc.current {
					t.Error("request method or GitHub headers do not match")
				}
				if tc.code != 0 {
					w.WriteHeader(tc.code)
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			result, err := Check(context.Background(), server.Client(), server.URL, tc.current)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result != (Result{Current: tc.current, Latest: "v1.10.0", URL: releaseURL, Status: tc.wantStatus}) {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
