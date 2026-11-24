// Package config loads and validates the goonhub-worker YAML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"goonhub/pkg/workerproto"
)

// Config is the whole worker configuration (see config.example.yaml for the documented
// keys and defaults).
type Config struct {
	// Server is the GoonHub server base URL, usually its Tailscale MagicDNS name
	// ("http://goonhub.tailnet.ts.net:8080").
	Server string `yaml:"server"`
	// Token is the worker token created in the GoonHub admin UI ("ghw_<id>_<secret>").
	// TokenFile reads it from a file instead (first line), which keeps it out of the
	// config file.
	Token     string `yaml:"token"`
	TokenFile string `yaml:"token_file"`
	// Name is the display name (default: the hostname).
	Name string `yaml:"name"`
	// Mode is "pull" (the worker claims jobs) or "push" (the server assigns jobs to the
	// worker's HTTP endpoint).
	Mode workerproto.ConnectionMode `yaml:"mode"`
	Push PushConfig                 `yaml:"push"`

	// Phases maps each phase the worker runs to its number of concurrent jobs. A phase
	// absent or at 0 is not processed by this worker.
	Phases map[workerproto.Phase]int `yaml:"phases"`
	// MaxConcurrency caps the jobs computing at once, all phases together (0 = sum of the
	// phase slots).
	MaxConcurrency int `yaml:"max_concurrency"`
	// MaxPendingUploads caps the computed jobs waiting for upload/complete; the worker
	// stops claiming above it (0 = 2 x max_concurrency).
	MaxPendingUploads int `yaml:"max_pending_uploads"`
	// SpritesConcurrency overrides the parallel frame extractions of sprite jobs
	// (0 = keep the server value).
	SpritesConcurrency int `yaml:"sprites_concurrency"`

	// Storage lists the server storage paths this machine can read directly (direct
	// input mode). Everything else is read over HTTP (indirect mode).
	Storage []StorageMapping `yaml:"storage"`

	// DataDir holds the outbox (job state and computed artifacts) and the logs
	// (default: "data" next to the config file).
	DataDir string      `yaml:"data_dir"`
	Cache   CacheConfig `yaml:"cache"`
	// MinFreeDisk: below this free space on the data or cache disk the worker stops
	// claiming (default 2GiB).
	MinFreeDisk ByteSize `yaml:"min_free_disk"`

	Bandwidth BandwidthConfig `yaml:"bandwidth"`
	Tools     ToolsConfig     `yaml:"tools"`
	// Accel is the hardware acceleration choice: "auto" (default), "none", or a method to
	// force (nvidia, videotoolbox, intel, vaapi, d3d11va, amd).
	Accel string `yaml:"accel"`
	// VAAPIDevice is the DRM render node used for VA-API (Linux, default
	// /dev/dri/renderD128).
	VAAPIDevice string         `yaml:"vaapi_device"`
	Timeouts    TimeoutsConfig `yaml:"timeouts"`
	// ShutdownGrace is how long a graceful stop lets computed jobs finish their upload
	// and complete before releasing them (default 10s).
	ShutdownGrace Duration  `yaml:"shutdown_grace"`
	Log           LogConfig `yaml:"log"`

	// path is the file the config was loaded from.
	path string
}

// PushConfig is the push-mode HTTP endpoint.
type PushConfig struct {
	// Listen is the listen address (":9090", "100.101.102.103:9090").
	Listen string `yaml:"listen"`
	// URL is the base URL the server uses to reach the worker
	// ("http://gaming-pc.tailnet.ts.net:9090"); default http://<hostname><listen port>.
	URL string `yaml:"url"`
	// Secret is the push token the server must send; empty = a random one per start
	// (the worker sends it to the server at register).
	Secret string `yaml:"secret"`
	// AllowedCIDRs are the source addresses accepted (default: Tailscale + loopback).
	AllowedCIDRs []string `yaml:"allowed_cidrs"`
}

// StorageMapping maps one server storage path to its local read-only mount. A job uses
// the mapping when its storage_path_id matches (then relative_path is joined to
// LocalPath) or when its server path starts with ServerPrefix.
type StorageMapping struct {
	StoragePathID uint   `yaml:"storage_path_id"`
	ServerPrefix  string `yaml:"server_prefix"`
	LocalPath     string `yaml:"local_path"`
}

// CacheConfig is the download cache of indirect mode.
type CacheConfig struct {
	Dir     string   `yaml:"dir"`      // default <data_dir>/cache
	MaxSize ByteSize `yaml:"max_size"` // default 20GiB
}

// BandwidthConfig limits the worker's own transfers (0 = unlimited). ffmpeg reading a
// media URL directly (sprites, animated thumbnails in indirect mode) is not limited.
type BandwidthConfig struct {
	Download ByteSize `yaml:"download"` // bytes per second
	Upload   ByteSize `yaml:"upload"`   // bytes per second
}

// ToolsConfig are the binaries (empty = auto-detected next to the worker binary, then
// on PATH).
type ToolsConfig struct {
	FFmpeg  string `yaml:"ffmpeg"`
	FFprobe string `yaml:"ffprobe"`
	Fpcalc  string `yaml:"fpcalc"`
}

// TimeoutsConfig are the worker-side job timeouts (the server's defaults).
type TimeoutsConfig struct {
	Sprites            Duration `yaml:"sprites"`             // default 30m
	AnimatedThumbnails Duration `yaml:"animated_thumbnails"` // default 30m (each clip also has the server's marker_timeout_sec)
	// Fingerprint: the job gets max(this, 0.5 s per video second) (default 5m).
	Fingerprint Duration `yaml:"fingerprint"`
}

// LogConfig is the logging setup.
type LogConfig struct {
	Level string `yaml:"level"` // debug, info (default), warn, error
	// File is the log file (default <data_dir>/logs/worker.log; "-" = console only).
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"` // rotation size (default 20)
	MaxBackups int    `yaml:"max_backups"` // rotated files kept (default 5)
	Format     string `yaml:"format"`      // console format: text (default) or json
	FileFormat string `yaml:"file_format"` // file format: json (default) or text
}

// DefaultTailscaleCIDRs are the source ranges accepted on the push endpoint by default.
var DefaultTailscaleCIDRs = []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48", "127.0.0.0/8", "::1/128"}

// Load reads, defaults and validates a config file.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}
	cfg, err := Parse(raw, path)
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

// Parse decodes a config document; path is the file it came from (relative paths in the
// config are relative to its directory).
func Parse(raw []byte, path string) (*Config, error) {
	cfg := &Config{}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	cfg.path = path
	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Path returns the file the config was loaded from.
func (c *Config) Path() string { return c.path }

func (c *Config) baseDir() string {
	if c.path == "" {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(c.path)
}

func (c *Config) resolve(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.baseDir(), p)
}

func (c *Config) applyDefaults() error {
	if c.Mode == "" {
		c.Mode = workerproto.ConnPull
	}
	if c.Name == "" {
		if h, err := os.Hostname(); err == nil {
			c.Name = h
		}
	}
	if c.Phases == nil {
		c.Phases = map[workerproto.Phase]int{
			workerproto.PhaseSprites:            1,
			workerproto.PhaseAnimatedThumbnails: 1,
			workerproto.PhaseFingerprint:        1,
		}
	}
	if c.MaxConcurrency == 0 {
		for _, n := range c.Phases {
			if n > 0 {
				c.MaxConcurrency += n
			}
		}
	}
	if c.MaxPendingUploads == 0 {
		c.MaxPendingUploads = 2 * c.MaxConcurrency
	}
	if c.TokenFile != "" {
		c.TokenFile = c.resolve(c.TokenFile)
		if c.Token == "" {
			b, err := os.ReadFile(c.TokenFile)
			if err != nil {
				return fmt.Errorf("failed to read token_file: %w", err)
			}
			c.Token = strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
		}
	}
	if env := os.Getenv("GOONHUB_WORKER_TOKEN"); env != "" && c.Token == "" {
		c.Token = env
	}
	c.DataDir = c.resolve(c.DataDir)
	if c.DataDir == "" {
		c.DataDir = filepath.Join(c.baseDir(), "data")
	}
	c.Cache.Dir = c.resolve(c.Cache.Dir)
	if c.Cache.Dir == "" {
		c.Cache.Dir = filepath.Join(c.DataDir, "cache")
	}
	if c.Cache.MaxSize == 0 {
		c.Cache.MaxSize = 20 << 30
	}
	if c.MinFreeDisk == 0 {
		c.MinFreeDisk = 2 << 30
	}
	c.Tools.FFmpeg = c.resolve(c.Tools.FFmpeg)
	c.Tools.FFprobe = c.resolve(c.Tools.FFprobe)
	c.Tools.Fpcalc = c.resolve(c.Tools.Fpcalc)
	if c.Accel == "" {
		c.Accel = "auto"
	}
	if c.Timeouts.Sprites == 0 {
		c.Timeouts.Sprites = Duration(30 * time.Minute)
	}
	if c.Timeouts.AnimatedThumbnails == 0 {
		c.Timeouts.AnimatedThumbnails = Duration(30 * time.Minute)
	}
	if c.Timeouts.Fingerprint == 0 {
		c.Timeouts.Fingerprint = Duration(5 * time.Minute)
	}
	if c.ShutdownGrace == 0 {
		c.ShutdownGrace = Duration(10 * time.Second)
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.File == "" {
		c.Log.File = filepath.Join(c.DataDir, "logs", "worker.log")
	} else if c.Log.File != "-" {
		c.Log.File = c.resolve(c.Log.File)
	}
	if c.Log.MaxSizeMB == 0 {
		c.Log.MaxSizeMB = 20
	}
	if c.Log.MaxBackups == 0 {
		c.Log.MaxBackups = 5
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
	if c.Log.FileFormat == "" {
		c.Log.FileFormat = "json"
	}
	if c.Mode == workerproto.ConnPush {
		if c.Push.Listen == "" {
			c.Push.Listen = ":9090"
		}
		if len(c.Push.AllowedCIDRs) == 0 {
			c.Push.AllowedCIDRs = DefaultTailscaleCIDRs
		}
		if c.Push.URL == "" {
			_, port, err := net.SplitHostPort(c.Push.Listen)
			if err != nil {
				return fmt.Errorf("push.listen %q: %w", c.Push.Listen, err)
			}
			host, err := os.Hostname()
			if err != nil {
				return fmt.Errorf("push.url is empty and the hostname is unknown: %w", err)
			}
			c.Push.URL = "http://" + strings.ToLower(host) + ":" + port
		}
		c.Push.URL = strings.TrimRight(c.Push.URL, "/")
	}
	c.Server = strings.TrimRight(strings.TrimSpace(c.Server), "/")
	for i := range c.Storage {
		c.Storage[i].LocalPath = c.resolve(c.Storage[i].LocalPath)
		c.Storage[i].ServerPrefix = strings.TrimRight(c.Storage[i].ServerPrefix, "/")
	}
	return nil
}

// Validate checks the config after defaults.
func (c *Config) Validate() error {
	var errs []error
	if c.Server == "" {
		errs = append(errs, errors.New("server is required (the GoonHub URL, e.g. http://goonhub.tailnet.ts.net:8080)"))
	} else if u, err := url.Parse(c.Server); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("server %q must be an http(s) URL", c.Server))
	}
	if c.Token == "" {
		errs = append(errs, errors.New("token is required (create the worker in GoonHub: Settings > Workers > Add worker), or set token_file / GOONHUB_WORKER_TOKEN"))
	} else if strings.IndexFunc(c.Token, unicode.IsSpace) >= 0 {
		errs = append(errs, errors.New("token must not contain spaces"))
	}
	if !c.Mode.Valid() {
		errs = append(errs, fmt.Errorf("mode %q must be pull or push", c.Mode))
	}
	total := 0
	for p, n := range c.Phases {
		if !p.Valid() {
			errs = append(errs, fmt.Errorf("phases: unknown phase %q (sprites, animated_thumbnails, fingerprint)", p))
		}
		if n < 0 {
			errs = append(errs, fmt.Errorf("phases: %s slots must be >= 0", p))
		}
		total += n
	}
	if total == 0 {
		errs = append(errs, errors.New("phases: at least one phase needs a slot"))
	}
	if c.MaxConcurrency < 1 {
		errs = append(errs, errors.New("max_concurrency must be >= 1"))
	}
	if c.MaxPendingUploads < 1 {
		errs = append(errs, errors.New("max_pending_uploads must be >= 1"))
	}
	if !validAccel(c.Accel) {
		errs = append(errs, fmt.Errorf("accel %q: use auto, none, nvidia, videotoolbox, intel, vaapi, d3d11va or amd", c.Accel))
	}
	if c.Mode == workerproto.ConnPush {
		if _, _, err := net.SplitHostPort(c.Push.Listen); err != nil {
			errs = append(errs, fmt.Errorf("push.listen %q: %w", c.Push.Listen, err))
		}
		if u, err := url.Parse(c.Push.URL); err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
			errs = append(errs, fmt.Errorf("push.url %q must be an http(s) URL", c.Push.URL))
		}
		for _, cidr := range c.Push.AllowedCIDRs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				errs = append(errs, fmt.Errorf("push.allowed_cidrs: %w", err))
			}
		}
	}
	for i, m := range c.Storage {
		if m.LocalPath == "" {
			errs = append(errs, fmt.Errorf("storage[%d]: local_path is required", i))
		}
		if m.StoragePathID == 0 && m.ServerPrefix == "" {
			errs = append(errs, fmt.Errorf("storage[%d]: set storage_path_id or server_prefix", i))
		}
		// The worker writes into data_dir and the cache: they must never be inside a
		// media mount.
		for _, own := range []struct{ key, dir string }{{"data_dir", c.DataDir}, {"cache.dir", c.Cache.Dir}} {
			if m.LocalPath != "" && isWithin(own.dir, m.LocalPath) {
				errs = append(errs, fmt.Errorf("%s %s is inside the media mount %s: use a separate directory", own.key, own.dir, m.LocalPath))
			}
		}
	}
	return errors.Join(errs...)
}

// isWithin reports whether path is root or inside it (lexically, case-insensitive on
// Windows-style paths).
func isWithin(path, root string) bool {
	p := filepath.Clean(path)
	r := filepath.Clean(root)
	if filepath.Separator == '\\' {
		p, r = strings.ToLower(p), strings.ToLower(r)
	}
	if p == r {
		return true
	}
	rel, err := filepath.Rel(r, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// EnabledPhases lists the phases with at least one slot, in protocol order.
func (c *Config) EnabledPhases() []workerproto.Phase {
	var out []workerproto.Phase
	for _, p := range workerproto.RemotePhases {
		if c.Phases[p] > 0 {
			out = append(out, p)
		}
	}
	return out
}

// OutboxDir is where job state lives.
func (c *Config) OutboxDir() string { return filepath.Join(c.DataDir, "jobs") }

// ---------------------------------------------------------------------------------------
// Value types
// ---------------------------------------------------------------------------------------

// Duration is a time.Duration written as "10s", "5m"... in YAML.
type Duration time.Duration

// D returns the time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// UnmarshalYAML parses a Go duration string (or a number of seconds).
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	s := strings.TrimSpace(n.Value)
	if s == "" {
		*d = 0
		return nil
	}
	if secs, err := strconv.ParseFloat(s, 64); err == nil {
		*d = Duration(time.Duration(secs * float64(time.Second)))
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: invalid duration %q (e.g. 30s, 5m, 1h)", n.Line, s)
	}
	*d = Duration(v)
	return nil
}

// ByteSize is a size in bytes written as "20GiB", "500MB", "10M", "1048576".
type ByteSize int64

// UnmarshalYAML parses a size.
func (b *ByteSize) UnmarshalYAML(n *yaml.Node) error {
	v, err := ParseByteSize(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*b = ByteSize(v)
	return nil
}

// ParseByteSize parses "20GiB", "500MB", "10M", "64k", "1048576" (K/M/G/T are binary
// multiples, as are KiB/MiB...; KB/MB/GB/TB are decimal).
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil || num < 0 {
		return 0, fmt.Errorf("invalid size %q (e.g. 20GiB, 500MB, 10M)", s)
	}
	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	unit = strings.TrimSuffix(unit, "/s")
	mult := map[string]float64{
		"": 1, "b": 1,
		"k": 1 << 10, "kib": 1 << 10, "kb": 1e3,
		"m": 1 << 20, "mib": 1 << 20, "mb": 1e6,
		"g": 1 << 30, "gib": 1 << 30, "gb": 1e9,
		"t": 1 << 40, "tib": 1 << 40, "tb": 1e12,
	}[unit]
	if mult == 0 {
		return 0, fmt.Errorf("invalid size unit in %q (B, K/KiB, M/MiB, G/GiB, T/TiB, KB, MB, GB, TB)", s)
	}
	return int64(num * mult), nil
}

// FormatBytes renders a byte count for humans.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// validAccel lists the accel values (the methods of goonhub/pkg/compute/hwaccel).
func validAccel(s string) bool {
	switch s {
	case "auto", "none", "nvidia", "videotoolbox", "intel", "vaapi", "d3d11va", "amd":
		return true
	}
	return false
}
