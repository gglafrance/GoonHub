package core

import (
	"fmt"
	"net/url"
	"strings"

	"goonhub/internal/apperrors"
)

// MaxSourceURLLen is the longest source URL a scene accepts.
const MaxSourceURLLen = 2048

// NormalizeSourceURL trims a scene's source URL and checks it. An empty value
// means "no source"; anything else must be an absolute http(s) URL without
// whitespace.
func NormalizeSourceURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len(value) > MaxSourceURLLen {
		return "", apperrors.NewValidationErrorWithField("source_url",
			fmt.Sprintf("source_url must be at most %d characters", MaxSourceURLLen))
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		strings.ContainsAny(value, " \t\r\n") {
		return "", apperrors.NewValidationErrorWithField("source_url", "source_url must be an http:// or https:// URL")
	}
	return value, nil
}
