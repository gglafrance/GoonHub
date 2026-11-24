package client

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	u := "http://h:8080/api/v1/workers/media/j1?exp=1&g=3&sig=abcdef0123"
	if got := RedactURL(u); strings.Contains(got, "abcdef") || !strings.Contains(got, "sig=REDACTED") {
		t.Fatalf("RedactURL: %s", got)
	}
	txt := "ffmpeg failed: " + u + ": Server returned 403"
	if got := RedactText(txt); strings.Contains(got, "abcdef") || !strings.Contains(got, "g=3&sig=REDACTED") {
		t.Fatalf("RedactText: %s", got)
	}
}
