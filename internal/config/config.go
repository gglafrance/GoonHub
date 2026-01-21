package config

import (
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
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
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

// Load reads configuration from file or environment variables.
func Load(path string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("environment", "development")
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)
	v.SetDefault("server.idle_timeout", 60*time.Second)
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

	return &cfg, nil
}
