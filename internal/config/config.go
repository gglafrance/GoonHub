package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment   string              `mapstructure:"environment"`
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Log           LogConfig           `mapstructure:"log"`
	Processing    ProcessingConfig    `mapstructure:"processing"`
	Auth          AuthConfig          `mapstructure:"auth"`
	Meilisearch   MeilisearchConfig   `mapstructure:"meilisearch"`
	PornDB        PornDBConfig        `mapstructure:"porndb"`
	Shutdown      ShutdownConfig      `mapstructure:"shutdown"`
	Streaming     StreamingConfig     `mapstructure:"streaming"`
	Pagination    PaginationConfig    `mapstructure:"pagination"`
	Playlists     PlaylistsConfig     `mapstructure:"playlists"`
	Sharing       SharingConfig       `mapstructure:"sharing"`
	Duplication   DuplicationConfig   `mapstructure:"duplication"`
	ClickHouse    ClickHouseConfig    `mapstructure:"clickhouse"`
	LibraryHealth LibraryHealthConfig `mapstructure:"library_health"`
	Workers       WorkersConfig       `mapstructure:"workers"`
}

// LibraryHealthConfig configures the background tasks of the library health page.
// WorkersConfig configures the remote workers (goonhub-worker, docs/REMOTE_WORKERS.md).
type WorkersConfig struct {
	Enabled              bool          `mapstructure:"enabled"`                 // enables the worker API
	HeartbeatInterval    time.Duration `mapstructure:"heartbeat_interval"`      // sent to the workers
	LeaseDuration        time.Duration `mapstructure:"lease_duration"`          // a lease not renewed for this long expires (>= 3 x heartbeat)
	ExpirySweepInterval  time.Duration `mapstructure:"expiry_sweep_interval"`   // how often expired leases are swept
	ClaimMaxWait         time.Duration `mapstructure:"claim_max_wait"`          // longest claim long-poll
	MediaURLTTL          time.Duration `mapstructure:"media_url_ttl"`           // validity of a signed media URL
	MediaSecret          string        `mapstructure:"media_secret"`            // HMAC secret of media URLs ("" = generated and stored in the database)
	MaxArtifactBytes     int64         `mapstructure:"max_artifact_bytes"`      // largest artifact
	UploadChunkBytes     int64         `mapstructure:"upload_chunk_bytes"`      // recommended PUT body size
	MaxCompleteBodyBytes int64         `mapstructure:"max_complete_body_bytes"` // largest complete body
	StagingRetention     time.Duration `mapstructure:"staging_retention"`       // abandoned staging dirs are deleted after this
	MaxAssignmentAge     time.Duration `mapstructure:"max_assignment_age"`      // a lease older than this is no longer renewed
	StartupLeaseGrace    time.Duration `mapstructure:"startup_lease_grace"`     // remote leases extended by this at start (0 = lease_duration)
	PushPollInterval     time.Duration `mapstructure:"push_poll_interval"`      // push dispatcher loop
	PushTimeout          time.Duration `mapstructure:"push_timeout"`            // timeout of a call to a push worker
	OfflineAfter         time.Duration `mapstructure:"offline_after"`           // no contact for this long = offline (0 = 3 x heartbeat)
	LocalPhases          []string      `mapstructure:"local_phases"`            // remotable phases the local pools still process
	AllowedCIDRs         []string      `mapstructure:"allowed_cidrs"`           // source addresses accepted on the worker API
	PublicURL            string        `mapstructure:"public_url"`              // base URL of the server for workers ("" = the URL the worker used)
}

type LibraryHealthConfig struct {
	// StatConcurrency is the number of files stat'ed at once by the missing-file check and the
	// fix-sizes task. Keep it low on NFS, where each stat is a network round trip.
	StatConcurrency int `mapstructure:"stat_concurrency"`
}

type DuplicationConfig struct {
	Enabled                 bool          `mapstructure:"enabled"`
	FingerprintWorkers      int           `mapstructure:"fingerprint_workers"`
	FingerprintTimeout      time.Duration `mapstructure:"fingerprint_timeout"`
	AudioDensityThreshold   float64       `mapstructure:"audio_density_threshold"`
	AudioMinHashes          int           `mapstructure:"audio_min_hashes"`
	AudioMaxHashOccurrences int           `mapstructure:"audio_max_hash_occurrences"`
	AudioMinSpan            int           `mapstructure:"audio_min_span"`
	VisualHammingMax        int           `mapstructure:"visual_hamming_max"`
	VisualMinFrames         int           `mapstructure:"visual_min_frames"`
	VisualMinSpan           int           `mapstructure:"visual_min_span"`
	DeltaTolerance          int           `mapstructure:"delta_tolerance"`
}

type ClickHouseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type SharingConfig struct {
	BaseURL string `mapstructure:"base_url"` // Public base URL for share links (e.g., "https://share.goonhub.example.com")
	Port    string `mapstructure:"port"`     // Port for the dedicated share server (empty = disabled)
}

type PaginationConfig struct {
	MaxItemsPerPage int `mapstructure:"max_items_per_page"`
}

// PlaylistsConfig configures the playlists (marker playlists: docs/MARKER_PLAYLISTS.md).
type PlaylistsConfig struct {
	// MaxMarkers is the most markers a marker playlist holds (0 = no limit).
	MaxMarkers int `mapstructure:"max_markers"`
	// MarkerDefaultDuration is how long, in seconds, a marker without an end plays in a marker
	// playlist before the next one.
	MarkerDefaultDuration int `mapstructure:"marker_default_duration"`
}

type StreamingConfig struct {
	MaxGlobalStreams int           `mapstructure:"max_global_streams"`
	MaxStreamsPerIP  int           `mapstructure:"max_streams_per_ip"`
	BufferSize       int           `mapstructure:"buffer_size"`
	PathCacheTTL     time.Duration `mapstructure:"path_cache_ttl"`
	PathCacheMaxSize int           `mapstructure:"path_cache_max_size"`
	// On-the-fly conversion of files browsers can't play (see streaming.Transcoder).
	Transcode TranscodeConfig `mapstructure:"transcode"`
}

// TranscodeConfig limits and tunes the on-the-fly conversion (CPU ffmpeg, H.264 + AAC in
// fragmented MP4) of scenes browsers can't play. It can be turned off at runtime with the
// transcoding_enabled app setting.
type TranscodeConfig struct {
	// Maximum number of ffmpeg conversions running at the same time (one per viewer without
	// the cache; with it, viewers of a part already converted or being converted need none).
	MaxConcurrent int `mapstructure:"max_concurrent"`
	// libx264 preset (ultrafast ... veryslow).
	Preset string `mapstructure:"preset"`
	// libx264 constant rate factor (0-51, lower is better quality).
	CRF int `mapstructure:"crf"`
	// Downscale videos larger than this (height of landscape videos, width of portrait
	// ones). 0 keeps the original size.
	MaxHeight int `mapstructure:"max_height"`
	// AAC bitrate of re-encoded audio, e.g. "160k".
	AudioBitrate string `mapstructure:"audio_bitrate"`
	// ffmpeg threads per conversion, 0 lets ffmpeg decide.
	Threads int `mapstructure:"threads"`
	// Seconds between keyframes of re-encoded video (one MP4 fragment per keyframe).
	KeyframeInterval int `mapstructure:"keyframe_interval"`
	// Cache of the converted output (see streaming.TranscodeCache).
	Cache TranscodeCacheConfig `mapstructure:"cache"`
}

// TranscodeCacheConfig keeps what on-the-fly conversions produced so that a replay, a seek
// back or a second viewer of the same part does not run ffmpeg again. The directory holds
// only GoonHub's own output: never point it at a media folder.
type TranscodeCacheConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Directory of the cached conversions (created if missing).
	Dir string `mapstructure:"dir"`
	// Total size above which the least recently used conversions are removed.
	MaxSizeGB float64 `mapstructure:"max_size_gb"`
	// Conversions not used for this long are removed.
	MaxAge time.Duration `mapstructure:"max_age"`
	// How often the size and age limits are applied (also done at startup).
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	// How far (in video time) a conversion may run ahead of its furthest viewer.
	ReadAhead time.Duration `mapstructure:"read_ahead"`
	// How long a conversion nobody watches anymore keeps running (a viewer seeking back
	// into it, or coming back, picks it up again) before ffmpeg is stopped. An idle
	// conversion is stopped at once when its slot is needed by another one.
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`
}

type PornDBConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type ShutdownConfig struct {
	GracefulTimeout   time.Duration `mapstructure:"graceful_timeout"`    // Total shutdown time (default: 30s)
	JobCompletionWait time.Duration `mapstructure:"job_completion_wait"` // Wait for running jobs (default: 15s)
	OrphanTimeout     time.Duration `mapstructure:"orphan_timeout"`      // Orphan detection threshold (default: 30s)
	StuckPendingTime  time.Duration `mapstructure:"stuck_pending_time"`  // Pending job stuck threshold (default: 10m)
}

type MeilisearchConfig struct {
	Host      string `mapstructure:"host"`
	APIKey    string `mapstructure:"api_key"`
	IndexName string `mapstructure:"index_name"`
}

type ServerConfig struct {
	Port           string        `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
	TLSCertFile    string        `mapstructure:"tls_cert_file"`   // Path to TLS certificate file
	TLSKeyFile     string        `mapstructure:"tls_key_file"`    // Path to TLS private key file
	TrustedProxies []string      `mapstructure:"trusted_proxies"` // CIDR ranges for trusted proxies (for X-Forwarded-For)
	SecureCookies  *bool         `mapstructure:"secure_cookies"`  // Override Secure flag on cookies (nil = auto from environment)
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json or console
}

type ProcessingConfig struct {
	FrameInterval               int           `mapstructure:"frame_interval"`                 // seconds
	MaxFrameDimension           int           `mapstructure:"max_frame_dimension"`            // longest side in pixels (small thumbnail)
	MaxFrameDimensionLarge      int           `mapstructure:"max_frame_dimension_large"`      // longest side in pixels (large thumbnail)
	FrameQuality                int           `mapstructure:"frame_quality"`                  // 1-100, WebP quality (small thumbnails)
	FrameQualityLg              int           `mapstructure:"frame_quality_lg"`               // 1-100, WebP quality (large thumbnails)
	FrameQualitySprites         int           `mapstructure:"frame_quality_sprites"`          // 1-100, WebP quality (sprite sheets)
	MetadataWorkers             int           `mapstructure:"metadata_workers"`               // concurrent metadata jobs
	ThumbnailWorkers            int           `mapstructure:"thumbnail_workers"`              // concurrent thumbnail jobs
	SpritesWorkers              int           `mapstructure:"sprites_workers"`                // concurrent sprites jobs
	ThumbnailSeek               string        `mapstructure:"thumbnail_seek"`                 // "00:00:05" or "5%"
	VideoDir                    string        `mapstructure:"video_dir"`                      // directory for video files
	MetadataDir                 string        `mapstructure:"metadata_dir"`                   // base directory for metadata (thumbnails, sprites, vtt)
	FrameOutputDir              string        `mapstructure:"frame_output_dir"`               // relative to app root
	ThumbnailDir                string        `mapstructure:"thumbnail_dir"`                  // relative to app root
	SpriteDir                   string        `mapstructure:"sprite_dir"`                     // relative to app root
	VttDir                      string        `mapstructure:"vtt_dir"`                        // relative to app root
	ActorImageDir               string        `mapstructure:"actor_image_dir"`                // directory for actor images
	StudioLogoDir               string        `mapstructure:"studio_logo_dir"`                // directory for studio logos
	MarkerThumbnailDir          string        `mapstructure:"marker_thumbnail_dir"`           // directory for marker thumbnails
	GridCols                    int           `mapstructure:"grid_cols"`                      // number of columns in sprite sheet
	GridRows                    int           `mapstructure:"grid_rows"`                      // number of rows in sprite sheet
	SpritesConcurrency          int           `mapstructure:"sprites_concurrency"`            // concurrent ffmpeg processes for sprite extraction (0 = auto)
	AnimatedThumbnailsWorkers   int           `mapstructure:"animated_thumbnails_workers"`    // concurrent animated thumbnail jobs
	AnimatedThumbnailsTimeout   time.Duration `mapstructure:"animated_thumbnails_timeout"`    // timeout for animated thumbnail jobs
	MarkerThumbnailType         string        `mapstructure:"marker_thumbnail_type"`          // "static" or "animated"
	MarkerAnimatedDuration      int           `mapstructure:"marker_animated_duration"`       // animated clip duration in seconds (3-15)
	MarkerThumbnailWorkers      int           `mapstructure:"marker_thumbnail_workers"`       // background marker thumbnail workers (default 2)
	ScenePreviewEnabled         bool          `mapstructure:"scene_preview_enabled"`          // enable scene preview video generation
	ScenePreviewSegments        int           `mapstructure:"scene_preview_segments"`         // number of segments to sample (2-24)
	ScenePreviewSegmentDuration float64       `mapstructure:"scene_preview_segment_duration"` // duration of each segment in seconds (0.75-30.0)
	ScenePreviewDir             string        `mapstructure:"scene_preview_dir"`              // directory for scene preview videos
	StagingDir                  string        `mapstructure:"staging_dir"`                    // job outputs before their commit ("" = "staging" next to sprite_dir)
	MarkerPreviewCRF            int           `mapstructure:"marker_preview_crf"`             // CRF for marker animated thumbnails (18-40)
	ScenePreviewCRF             int           `mapstructure:"scene_preview_crf"`              // CRF for scene preview videos (18-40)
	JobHistoryRetention         string        `mapstructure:"job_history_retention"`          // completed/cancelled jobs kept this long, e.g. "30d", "24h" ("0" = never purged)
	JobHistoryFailedRetention   string        `mapstructure:"job_history_failed_retention"`   // failed/timed out jobs kept this long, e.g. "90d" ("0" = never purged)
	JobHistoryPurgeInterval     time.Duration `mapstructure:"job_history_purge_interval"`     // how often the job history purge runs
	JobHistoryPurgeBatchSize    int           `mapstructure:"job_history_purge_batch_size"`   // rows deleted per statement by the purge
	MetadataTimeout             time.Duration `mapstructure:"metadata_timeout"`               // timeout for metadata extraction jobs
	ThumbnailTimeout            time.Duration `mapstructure:"thumbnail_timeout"`              // timeout for thumbnail extraction jobs
	SpritesTimeout              time.Duration `mapstructure:"sprites_timeout"`                // timeout for sprite sheet generation jobs
	FailingThreshold            int           `mapstructure:"failing_threshold"`              // consecutive failures of a (scene, phase) on an unchanged file before it is no longer retried automatically (0 = never)
}

type AuthConfig struct {
	PasetoSecret       string        `mapstructure:"paseto_secret"`
	AdminUsername      string        `mapstructure:"admin_username"`
	AdminPassword      string        `mapstructure:"admin_password"`
	TokenDuration      time.Duration `mapstructure:"token_duration"`       // session lifetime (token + cookie), e.g. "24h", "30d", "1y"
	LoginRateLimit     int           `mapstructure:"login_rate_limit"`     // requests per minute
	LoginRateBurst     int           `mapstructure:"login_rate_burst"`     // burst size
	LockoutThreshold   int           `mapstructure:"lockout_threshold"`    // failed attempts before lockout
	LockoutDuration    time.Duration `mapstructure:"lockout_duration"`     // how long account is locked
	LockoutCleanupFreq time.Duration `mapstructure:"lockout_cleanup_freq"` // how often to cleanup old entries
}

// Load reads configuration from file or environment variables.
func Load(path string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("environment", "development")
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)
	v.SetDefault("server.idle_timeout", 60*time.Second)
	v.SetDefault("server.allowed_origins", []string{"http://localhost:3000"})
	v.SetDefault("server.tls_cert_file", "")    // Empty = TLS disabled
	v.SetDefault("server.tls_key_file", "")     // Empty = TLS disabled
	v.SetDefault("server.trusted_proxies", nil) // nil = trust no proxies; set to ["127.0.0.1", "::1"] for loopback or CIDR ranges
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "goonhub")
	v.SetDefault("database.password", "goonhub_dev_password")
	v.SetDefault("database.dbname", "goonhub")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("processing.frame_interval", 5)
	v.SetDefault("processing.max_frame_dimension", 320)
	v.SetDefault("processing.max_frame_dimension_large", 1280)
	v.SetDefault("processing.frame_quality", 85)
	v.SetDefault("processing.frame_quality_lg", 85)
	v.SetDefault("processing.frame_quality_sprites", 75)
	v.SetDefault("processing.metadata_workers", 3)
	v.SetDefault("processing.thumbnail_workers", 1)
	v.SetDefault("processing.sprites_workers", 1)
	v.SetDefault("processing.thumbnail_seek", "00:00:05")
	v.SetDefault("processing.video_dir", "./data/videos")
	v.SetDefault("processing.metadata_dir", "./data/metadata")
	v.SetDefault("processing.frame_output_dir", "./data/metadata/frames")
	v.SetDefault("processing.thumbnail_dir", "./data/metadata/thumbnails")
	v.SetDefault("processing.sprite_dir", "./data/metadata/sprites")
	v.SetDefault("processing.vtt_dir", "./data/metadata/vtt")
	v.SetDefault("processing.actor_image_dir", "./data/metadata/actors")
	v.SetDefault("processing.studio_logo_dir", "./data/metadata/studios")
	v.SetDefault("processing.marker_thumbnail_dir", "./data/metadata/marker-thumbnails")
	v.SetDefault("processing.grid_cols", 12)
	v.SetDefault("processing.grid_rows", 8)
	v.SetDefault("processing.sprites_concurrency", 0)
	v.SetDefault("processing.animated_thumbnails_workers", 1)
	v.SetDefault("processing.animated_thumbnails_timeout", 5*time.Minute)
	v.SetDefault("processing.marker_thumbnail_type", "static")
	v.SetDefault("processing.marker_animated_duration", 10)
	v.SetDefault("processing.scene_preview_enabled", false)
	v.SetDefault("processing.scene_preview_segments", 12)
	v.SetDefault("processing.scene_preview_segment_duration", 1.0)
	v.SetDefault("processing.scene_preview_dir", "./data/metadata/scene-previews")
	v.SetDefault("processing.staging_dir", "")
	v.SetDefault("processing.marker_preview_crf", 32)
	v.SetDefault("processing.scene_preview_crf", 27)
	v.SetDefault("processing.job_history_retention", "30d")
	v.SetDefault("processing.job_history_failed_retention", "90d")
	v.SetDefault("processing.job_history_purge_interval", time.Hour)
	v.SetDefault("processing.job_history_purge_batch_size", 5000)
	v.SetDefault("processing.failing_threshold", 3)
	v.SetDefault("processing.metadata_timeout", 5*time.Minute)
	v.SetDefault("processing.thumbnail_timeout", 2*time.Minute)
	v.SetDefault("processing.sprites_timeout", 30*time.Minute)
	v.SetDefault("auth.paseto_secret", "")
	v.SetDefault("auth.admin_username", "admin")
	v.SetDefault("auth.admin_password", "admin")
	v.SetDefault("auth.token_duration", 24*time.Hour)
	v.SetDefault("auth.login_rate_limit", 10)
	v.SetDefault("auth.login_rate_burst", 5)
	v.SetDefault("auth.lockout_threshold", 5)             // Lock after 5 failed attempts
	v.SetDefault("auth.lockout_duration", 15*time.Minute) // Lock for 15 minutes
	v.SetDefault("auth.lockout_cleanup_freq", 5*time.Minute)
	v.SetDefault("meilisearch.host", "http://localhost:7700")
	v.SetDefault("meilisearch.api_key", "goonhub_dev_master_key")
	v.SetDefault("meilisearch.index_name", "videos")
	v.SetDefault("porndb.api_key", "")
	v.SetDefault("shutdown.graceful_timeout", 30*time.Second)
	v.SetDefault("shutdown.job_completion_wait", 15*time.Second)
	v.SetDefault("shutdown.orphan_timeout", 30*time.Second)
	v.SetDefault("shutdown.stuck_pending_time", 10*time.Minute)
	v.SetDefault("pagination.max_items_per_page", 100)
	v.SetDefault("playlists.max_markers", 1000)
	v.SetDefault("playlists.marker_default_duration", 30)
	v.SetDefault("sharing.base_url", "")
	v.SetDefault("sharing.port", "")
	v.SetDefault("duplication.enabled", false)
	v.SetDefault("library_health.stat_concurrency", 8)
	v.SetDefault("workers.enabled", false)
	v.SetDefault("workers.heartbeat_interval", 10*time.Second)
	v.SetDefault("workers.lease_duration", 60*time.Second)
	v.SetDefault("workers.expiry_sweep_interval", 5*time.Second)
	v.SetDefault("workers.claim_max_wait", 25*time.Second)
	v.SetDefault("workers.media_url_ttl", 6*time.Hour)
	v.SetDefault("workers.media_secret", "")
	v.SetDefault("workers.max_artifact_bytes", int64(2<<30))
	v.SetDefault("workers.upload_chunk_bytes", int64(16<<20))
	v.SetDefault("workers.max_complete_body_bytes", int64(64<<20))
	v.SetDefault("workers.staging_retention", 24*time.Hour)
	v.SetDefault("workers.max_assignment_age", 24*time.Hour)
	v.SetDefault("workers.startup_lease_grace", time.Duration(0))
	v.SetDefault("workers.push_poll_interval", 5*time.Second)
	v.SetDefault("workers.push_timeout", 10*time.Second)
	v.SetDefault("workers.offline_after", time.Duration(0))
	v.SetDefault("workers.local_phases", []string{"sprites", "animated_thumbnails", "fingerprint"})
	v.SetDefault("workers.allowed_cidrs", []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48", "127.0.0.0/8", "::1/128"})
	v.SetDefault("workers.public_url", "")
	v.SetDefault("duplication.fingerprint_workers", 1)
	v.SetDefault("duplication.fingerprint_timeout", 5*time.Minute)
	v.SetDefault("duplication.audio_density_threshold", 0.50)
	v.SetDefault("duplication.audio_min_hashes", 80)
	v.SetDefault("duplication.audio_max_hash_occurrences", 10)
	v.SetDefault("duplication.audio_min_span", 160)
	v.SetDefault("duplication.visual_hamming_max", 5)
	v.SetDefault("duplication.visual_min_frames", 20)
	v.SetDefault("duplication.visual_min_span", 30)
	v.SetDefault("duplication.delta_tolerance", 2)
	v.SetDefault("clickhouse.host", "localhost")
	v.SetDefault("clickhouse.port", 9000)
	v.SetDefault("clickhouse.database", "goonhub")
	v.SetDefault("clickhouse.user", "default")
	v.SetDefault("clickhouse.password", "")
	v.SetDefault("streaming.max_global_streams", 100)
	v.SetDefault("streaming.max_streams_per_ip", 10)
	v.SetDefault("streaming.buffer_size", 262144) // 256KB (8x default 32KB)
	v.SetDefault("streaming.path_cache_ttl", 5*time.Minute)
	v.SetDefault("streaming.path_cache_max_size", 10000)
	v.SetDefault("streaming.transcode.max_concurrent", 4)
	v.SetDefault("streaming.transcode.preset", "veryfast")
	v.SetDefault("streaming.transcode.crf", 23)
	v.SetDefault("streaming.transcode.max_height", 1080)
	v.SetDefault("streaming.transcode.audio_bitrate", "160k")
	v.SetDefault("streaming.transcode.threads", 0)
	v.SetDefault("streaming.transcode.keyframe_interval", 2)
	v.SetDefault("streaming.transcode.cache.enabled", true)
	v.SetDefault("streaming.transcode.cache.dir", "./data/transcode-cache")
	v.SetDefault("streaming.transcode.cache.max_size_gb", 20)
	v.SetDefault("streaming.transcode.cache.max_age", 24*time.Hour)
	v.SetDefault("streaming.transcode.cache.cleanup_interval", 10*time.Minute)
	v.SetDefault("streaming.transcode.cache.read_ahead", 60*time.Second)
	v.SetDefault("streaming.transcode.cache.idle_timeout", 30*time.Second)

	// Environment variables
	v.SetEnvPrefix("GOONHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	// The session lifetime also accepts days and years ("30d", "1y"), which
	// time.ParseDuration does not know.
	if raw := strings.TrimSpace(v.GetString("auth.token_duration")); raw != "" {
		d, err := ParseSessionDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("auth.token_duration: %w", err)
		}
		v.Set("auth.token_duration", d)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.Auth.TokenDuration <= 0 {
		return nil, fmt.Errorf("auth.token_duration must be positive, got %s", cfg.Auth.TokenDuration)
	}

	// Validate PASETO secret
	if cfg.Auth.PasetoSecret == "" {
		if cfg.Environment == "production" {
			return nil, fmt.Errorf("GOONHUB_AUTH_PASETO_SECRET is required in production")
		}

		// Generate random key for development (tokens will not persist across restarts)
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate PASETO key: %w", err)
		}
		cfg.Auth.PasetoSecret = hex.EncodeToString(key)
		// Security: Never log the actual secret value
		fmt.Println("[WARNING] Generated ephemeral PASETO key for development - tokens will not survive server restart")
		fmt.Println("[WARNING] Set GOONHUB_AUTH_PASETO_SECRET environment variable for persistent sessions")
	}

	// Validate production security requirements
	if cfg.Environment == "production" {
		if err := validateAdminPassword(cfg.Auth.AdminPassword); err != nil {
			return nil, fmt.Errorf("GOONHUB_AUTH_ADMIN_PASSWORD: %w", err)
		}
		if cfg.Database.Password == "goonhub_dev_password" || cfg.Database.Password == "" {
			return nil, fmt.Errorf("GOONHUB_DATABASE_PASSWORD must be set to a secure value in production")
		}
		if cfg.Database.SSLMode == "disable" {
			fmt.Println("[WARNING] Database SSL is disabled in production - consider enabling for security")
		}
	} else {
		// Development warnings
		if cfg.Auth.AdminPassword == "admin" {
			fmt.Println("[WARNING] Using default admin password 'admin' - set GOONHUB_AUTH_ADMIN_PASSWORD for security")
		}
	}

	return &cfg, nil
}

// ParseRetentionDuration parses a retention duration string like "7d", "24h", "30m".
// Supports "d" suffix for days, otherwise falls back to time.ParseDuration.
func ParseRetentionDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 7 * 24 * time.Hour, nil
	}
	if daysStr, ok := strings.CutSuffix(s, "d"); ok {
		var days int
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// ParseSessionDuration parses a session lifetime. Besides the time.ParseDuration
// units it accepts a whole number of days ("30d") or years of 365 days ("1y").
func ParseSessionDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	for suffix, unit := range map[string]time.Duration{"d": 24 * time.Hour, "y": 365 * 24 * time.Hour} {
		numStr, ok := strings.CutSuffix(s, suffix)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * unit, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// validateAdminPassword checks that the admin password meets security requirements.
// Requirements:
// - Minimum 12 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
// - Not a common/default password
func validateAdminPassword(password string) error {
	if password == "" || password == "admin" {
		return fmt.Errorf("must be set to a secure value (not empty or 'admin')")
	}

	if len(password) < 12 {
		return fmt.Errorf("must be at least 12 characters (got %d)", len(password))
	}

	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("must contain at least one digit")
	}

	return nil
}
