package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSessionDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"24h":     24 * time.Hour,
		"90m":     90 * time.Minute,
		"30d":     30 * 24 * time.Hour,
		"1y":      365 * 24 * time.Hour,
		" 2y ":    2 * 365 * 24 * time.Hour,
		"8760h":   365 * 24 * time.Hour,
		"24h0m0s": 24 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseSessionDuration(in)
		if err != nil {
			t.Fatalf("ParseSessionDuration(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseSessionDuration(%q) = %s, want %s", in, got, want)
		}
	}
	for _, in := range []string{"", "abc", "1.5d", "d", "1w"} {
		if _, err := ParseSessionDuration(in); err == nil {
			t.Errorf("ParseSessionDuration(%q) should fail", in)
		}
	}
}

func loadWithTokenDuration(t *testing.T, value string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "auth:\n  paseto_secret: \"0123456789abcdef0123456789abcdef\"\n  token_duration: " + value + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestLoadTokenDuration(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.TokenDuration != 24*time.Hour {
		t.Errorf("default token_duration = %s, want 24h", cfg.Auth.TokenDuration)
	}

	cfg, err = loadWithTokenDuration(t, "1y")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.TokenDuration != 365*24*time.Hour {
		t.Errorf("token_duration 1y = %s", cfg.Auth.TokenDuration)
	}

	t.Setenv("GOONHUB_AUTH_TOKEN_DURATION", "30d")
	cfg, err = loadWithTokenDuration(t, "1y")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.TokenDuration != 30*24*time.Hour {
		t.Errorf("env token_duration 30d = %s", cfg.Auth.TokenDuration)
	}
}

func TestLoadRejectsInvalidTokenDuration(t *testing.T) {
	for _, value := range []string{"0s", "-1h", "soon"} {
		if _, err := loadWithTokenDuration(t, value); err == nil {
			t.Errorf("token_duration %q should be rejected", value)
		}
	}
}
