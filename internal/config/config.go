package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment string             `mapstructure:"environment"`
	Server      ServerConfig       `mapstructure:"server"`
	Database    DatabaseConfig     `mapstructure:"database"`
	Log         LogConfig          `mapstructure:"log"`
	Processing  ProcessingConfig   `mapstructure:"processing"`
	Auth        AuthConfig         `mapstructure:"auth"`
	Meilisearch MeilisearchConfig  `mapstructure:"meilisearch"`
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
	FrameInterval            int           `mapstructure:"frame_interval"`             // seconds
	MaxFrameDimension        int           `mapstructure:"max_frame_dimension"`        // longest side in pixels (small thumbnail)
	MaxFrameDimensionLarge   int           `mapstructure:"max_frame_dimension_large"`  // longest side in pixels (large thumbnail)
	FrameQuality             int           `mapstructure:"frame_quality"`              // 1-100, WebP quality (small thumbnails)
	FrameQualityLg           int           `mapstructure:"frame_quality_lg"`           // 1-100, WebP quality (large thumbnails)
	FrameQualitySprites      int           `mapstructure:"frame_quality_sprites"`      // 1-100, WebP quality (sprite sheets)
	MetadataWorkers          int           `mapstructure:"metadata_workers"`           // concurrent metadata jobs
	ThumbnailWorkers         int           `mapstructure:"thumbnail_workers"`          // concurrent thumbnail jobs
	SpritesWorkers           int           `mapstructure:"sprites_workers"`            // concurrent sprites jobs
	ThumbnailSeek            string        `mapstructure:"thumbnail_seek"`             // "00:00:05" or "5%"
	FrameOutputDir           string        `mapstructure:"frame_output_dir"`           // relative to app root
	ThumbnailDir             string        `mapstructure:"thumbnail_dir"`              // relative to app root
	SpriteDir                string        `mapstructure:"sprite_dir"`                 // relative to app root
	VttDir                   string        `mapstructure:"vtt_dir"`                    // relative to app root
	ActorImageDir            string        `mapstructure:"actor_image_dir"`            // directory for actor images
	GridCols                 int           `mapstructure:"grid_cols"`                  // number of columns in sprite sheet
	GridRows                 int           `mapstructure:"grid_rows"`                  // number of rows in sprite sheet
	SpritesConcurrency       int           `mapstructure:"sprites_concurrency"`        // concurrent ffmpeg processes for sprite extraction (0 = auto)
	JobHistoryRetention      string        `mapstructure:"job_history_retention"`      // duration string e.g. "7d", "24h"
	MetadataTimeout          time.Duration `mapstructure:"metadata_timeout"`           // timeout for metadata extraction jobs
	ThumbnailTimeout         time.Duration `mapstructure:"thumbnail_timeout"`          // timeout for thumbnail extraction jobs
	SpritesTimeout           time.Duration `mapstructure:"sprites_timeout"`            // timeout for sprite sheet generation jobs
}

type AuthConfig struct {
	PasetoSecret   string        `mapstructure:"paseto_secret"`
	AdminUsername  string        `mapstructure:"admin_username"`
	AdminPassword  string        `mapstructure:"admin_password"`
	TokenDuration  time.Duration `mapstructure:"token_duration"`
	LoginRateLimit int           `mapstructure:"login_rate_limit"` // requests per minute
	LoginRateBurst int           `mapstructure:"login_rate_burst"` // burst size
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
	v.SetDefault("processing.frame_output_dir", "./data/frames")
	v.SetDefault("processing.thumbnail_dir", "./data/thumbnails")
	v.SetDefault("processing.sprite_dir", "./data/sprites")
	v.SetDefault("processing.vtt_dir", "./data/vtt")
	v.SetDefault("processing.actor_image_dir", "./data/actors")
	v.SetDefault("processing.grid_cols", 12)
	v.SetDefault("processing.grid_rows", 8)
	v.SetDefault("processing.sprites_concurrency", 0)
	v.SetDefault("processing.job_history_retention", "7d")
	v.SetDefault("processing.metadata_timeout", 5*time.Minute)
	v.SetDefault("processing.thumbnail_timeout", 2*time.Minute)
	v.SetDefault("processing.sprites_timeout", 30*time.Minute)
	v.SetDefault("auth.paseto_secret", "")
	v.SetDefault("auth.admin_username", "admin")
	v.SetDefault("auth.admin_password", "admin")
	v.SetDefault("auth.token_duration", 24*time.Hour)
	v.SetDefault("auth.login_rate_limit", 10)
	v.SetDefault("auth.login_rate_burst", 5)
	v.SetDefault("meilisearch.host", "http://localhost:7700")
	v.SetDefault("meilisearch.api_key", "goonhub_dev_master_key")
	v.SetDefault("meilisearch.index_name", "videos")

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

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Validate PASETO secret
	if cfg.Auth.PasetoSecret == "" {
		if cfg.Environment == "production" {
			return nil, fmt.Errorf("GOONHUB_AUTH_PASETO_SECRET is required in production")
		}

		// Generate random key for development
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate PASETO key: %w", err)
		}
		cfg.Auth.PasetoSecret = hex.EncodeToString(key)
		fmt.Printf("[WARNING] Generated random PASETO key for development: %s\n", cfg.Auth.PasetoSecret)
		fmt.Println("[WARNING] Set GOONHUB_AUTH_PASETO_SECRET environment variable to use a persistent key")
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
