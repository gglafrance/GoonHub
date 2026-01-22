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
	Environment string           `mapstructure:"environment"`
	Server      ServerConfig     `mapstructure:"server"`
	Database    DatabaseConfig   `mapstructure:"database"`
	Log         LogConfig        `mapstructure:"log"`
	Processing  ProcessingConfig `mapstructure:"processing"`
	Auth        AuthConfig       `mapstructure:"auth"`
}

type ServerConfig struct {
	Port           string        `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	Source string `mapstructure:"source"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json or console
}

type ProcessingConfig struct {
	FrameInterval  int    `mapstructure:"frame_interval"`   // seconds
	FrameWidth     int    `mapstructure:"frame_width"`      // pixels
	FrameHeight    int    `mapstructure:"frame_height"`     // pixels
	FrameQuality   int    `mapstructure:"frame_quality"`    // 1-100, WebP quality
	WorkerCount    int    `mapstructure:"worker_count"`     // concurrent jobs
	ThumbnailSeek  string `mapstructure:"thumbnail_seek"`   // "00:00:05" or "5%"
	FrameOutputDir string `mapstructure:"frame_output_dir"` // relative to app root
	ThumbnailDir   string `mapstructure:"thumbnail_dir"`    // relative to app root
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
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.source", "library.db")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("processing.frame_interval", 60)
	v.SetDefault("processing.frame_width", 320)
	v.SetDefault("processing.frame_height", 180)
	v.SetDefault("processing.frame_quality", 85)
	v.SetDefault("processing.worker_count", 2)
	v.SetDefault("processing.thumbnail_seek", "00:00:05")
	v.SetDefault("processing.frame_output_dir", "./data/frames")
	v.SetDefault("processing.thumbnail_dir", "./data/thumbnails")
	v.SetDefault("auth.paseto_secret", "")
	v.SetDefault("auth.admin_username", "admin")
	v.SetDefault("auth.admin_password", "admin")
	v.SetDefault("auth.token_duration", 24*time.Hour)
	v.SetDefault("auth.login_rate_limit", 10)
	v.SetDefault("auth.login_rate_burst", 5)

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
