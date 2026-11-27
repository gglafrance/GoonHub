package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goonhub/pkg/workerproto"
)

func TestParseDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Parse([]byte("server: http://goonhub.tail.ts.net:8080/\ntoken: ghw_1_abc\n"), filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "http://goonhub.tail.ts.net:8080" {
		t.Fatalf("server %q", cfg.Server)
	}
	if cfg.Mode != workerproto.ConnPull || cfg.MaxConcurrency != 3 || cfg.MaxPendingUploads != 6 {
		t.Fatalf("defaults: mode %s concurrency %d pending %d", cfg.Mode, cfg.MaxConcurrency, cfg.MaxPendingUploads)
	}
	if cfg.DataDir != filepath.Join(dir, "data") || cfg.Cache.Dir != filepath.Join(dir, "data", "cache") {
		t.Fatalf("dirs %s %s", cfg.DataDir, cfg.Cache.Dir)
	}
	if cfg.Cache.MaxSize != 20<<30 || cfg.ShutdownGrace.D() != 10*time.Second || cfg.Timeouts.Fingerprint.D() != 5*time.Minute {
		t.Fatalf("defaults: %+v", cfg)
	}
	if got := cfg.EnabledPhases(); len(got) != 3 {
		t.Fatalf("phases %v", got)
	}
}

func TestParseFull(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("ghw_2_secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	y := `
server: https://gh.example.ts.net
token_file: token
mode: push
push:
  listen: ":9191"
  url: http://worker.ts.net:9191/
phases:
  sprites: 2
  fingerprint: 0
max_concurrency: 2
storage:
  - storage_path_id: 3
    local_path: /mnt/stash
  - server_prefix: /media/other/
    local_path: /mnt/other
cache:
  max_size: 500MB
bandwidth:
  download: 10MiB
  upload: 2M
timeouts:
  sprites: 1h
shutdown_grace: 30
`
	cfg, err := Parse([]byte(y), filepath.Join(dir, "c.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "ghw_2_secret" {
		t.Fatalf("token %q", cfg.Token)
	}
	if cfg.Push.URL != "http://worker.ts.net:9191" || len(cfg.Push.AllowedCIDRs) != 4 {
		t.Fatalf("push %+v", cfg.Push)
	}
	if got := cfg.EnabledPhases(); len(got) != 1 || got[0] != workerproto.PhaseSprites {
		t.Fatalf("phases %v", got)
	}
	if cfg.Cache.MaxSize != 500e6 || cfg.Bandwidth.Download != 10<<20 || cfg.Bandwidth.Upload != 2<<20 {
		t.Fatalf("sizes %d %d %d", cfg.Cache.MaxSize, cfg.Bandwidth.Download, cfg.Bandwidth.Upload)
	}
	if cfg.Timeouts.Sprites.D() != time.Hour || cfg.ShutdownGrace.D() != 30*time.Second {
		t.Fatalf("durations %v %v", cfg.Timeouts.Sprites.D(), cfg.ShutdownGrace.D())
	}
	if cfg.Storage[1].ServerPrefix != "/media/other" {
		t.Fatalf("prefix %q", cfg.Storage[1].ServerPrefix)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name, yaml, want string
	}{
		{"no server", "token: x\n", "server is required"},
		{"bad server", "server: ftp://x\ntoken: x\n", "http(s) URL"},
		{"no token", "server: http://x\n", "token is required"},
		{"bad mode", "server: http://x\ntoken: x\nmode: poll\n", "pull or push"},
		{"bad phase", "server: http://x\ntoken: x\nphases:\n  thumbnail: 1\n", "unknown phase"},
		{"no slot", "server: http://x\ntoken: x\nphases:\n  sprites: 0\n", "at least one phase"},
		{"unknown key", "server: http://x\ntoken: x\nservr: y\n", "invalid YAML"},
		{"bad size", "server: http://x\ntoken: x\ncache:\n  max_size: 10 parsecs\n", "invalid size"},
		{"bad accel", "server: http://x\ntoken: x\naccel: turbo\n", "accel"},
		{"mapping without key", "server: http://x\ntoken: x\nstorage:\n  - local_path: /a\n", "storage_path_id or server_prefix"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml), filepath.Join(t.TempDir(), "c.yaml"))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
		})
	}
}

func TestDataDirInsideMediaMountRefused(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "media")
	y := "server: http://x\ntoken: x\ndata_dir: media/worker\nstorage:\n  - storage_path_id: 1\n    local_path: media\n"
	_, err := Parse([]byte(y), filepath.Join(dir, "c.yaml"))
	if err == nil || !strings.Contains(err.Error(), "inside the media mount") {
		t.Fatalf("error %v (media %s)", err, media)
	}
}

func TestParseByteSize(t *testing.T) {
	cases := map[string]int64{"": 0, "0": 0, "1024": 1024, "1K": 1024, "1KiB": 1024, "1KB": 1000, "1.5GiB": 3 << 29, "2 MB": 2e6, "10M/s": 10 << 20}
	for in, want := range cases {
		got, err := ParseByteSize(in)
		if err != nil || got != want {
			t.Fatalf("ParseByteSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"abc", "-1", "5 XB"} {
		if _, err := ParseByteSize(bad); err == nil {
			t.Fatalf("ParseByteSize(%q) accepted", bad)
		}
	}
}

func TestIsWithin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "a")
	if !isWithin(filepath.Join(root, "b"), root) || !isWithin(root, root) {
		t.Fatal("inside not detected")
	}
	if isWithin(root+"b", root) || isWithin(filepath.Dir(root), root) {
		t.Fatal("outside detected as inside")
	}
}

func TestExampleConfigParses(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	y := strings.Replace(string(raw), `token: ""`, `token: "ghw_1_x"`, 1)
	cfg, err := Parse([]byte(y), filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != workerproto.ConnPull || cfg.MaxConcurrency != 2 || cfg.Phases[workerproto.PhaseFingerprint] != 2 {
		t.Fatalf("example: %+v", cfg)
	}
}
