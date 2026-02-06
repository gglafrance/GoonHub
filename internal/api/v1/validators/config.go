package validators

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

// PoolConfigLimits defines the valid ranges for pool configuration
const (
	MinWorkers = 1
	MaxWorkers = 10
)

// RetryConfigLimits defines the valid ranges for retry configuration
const (
	MinMaxRetries          = 0
	MaxMaxRetries          = 10
	MinInitialDelaySeconds = 1
	MaxInitialDelaySeconds = 3600
	MaxDelaySecondsLimit   = 86400
	MinBackoffFactor       = 1.0
	MaxBackoffFactor       = 5.0
)

// ValidateWorkerCount validates a worker count is within acceptable range
func ValidateWorkerCount(count int, fieldName string) error {
	if count < MinWorkers || count > MaxWorkers {
		return fmt.Errorf("%s must be between %d and %d", fieldName, MinWorkers, MaxWorkers)
	}
	return nil
}

// PoolConfigInput represents the input for pool configuration validation
type PoolConfigInput struct {
	MetadataWorkers           int
	ThumbnailWorkers          int
	SpritesWorkers            int
	AnimatedThumbnailsWorkers int
}

// ValidatePoolConfig validates all pool configuration fields
func ValidatePoolConfig(cfg PoolConfigInput) error {
	if err := ValidateWorkerCount(cfg.MetadataWorkers, "metadata_workers"); err != nil {
		return err
	}
	if err := ValidateWorkerCount(cfg.ThumbnailWorkers, "thumbnail_workers"); err != nil {
		return err
	}
	if err := ValidateWorkerCount(cfg.SpritesWorkers, "sprites_workers"); err != nil {
		return err
	}
	if err := ValidateWorkerCount(cfg.AnimatedThumbnailsWorkers, "animated_thumbnails_workers"); err != nil {
		return err
	}
	return nil
}

// RetryConfigInput represents the input for retry configuration validation
type RetryConfigInput struct {
	Phase               string
	MaxRetries          int
	InitialDelaySeconds int
	MaxDelaySeconds     int
	BackoffFactor       float64
}

// ValidateRetryConfig validates all retry configuration fields
func ValidateRetryConfig(cfg RetryConfigInput) error {
	if err := ValidatePhase(cfg.Phase); err != nil {
		return err
	}
	if cfg.MaxRetries < MinMaxRetries || cfg.MaxRetries > MaxMaxRetries {
		return fmt.Errorf("max_retries must be between %d and %d", MinMaxRetries, MaxMaxRetries)
	}
	if cfg.InitialDelaySeconds < MinInitialDelaySeconds || cfg.InitialDelaySeconds > MaxInitialDelaySeconds {
		return fmt.Errorf("initial_delay_seconds must be between %d and %d", MinInitialDelaySeconds, MaxInitialDelaySeconds)
	}
	if cfg.MaxDelaySeconds < cfg.InitialDelaySeconds || cfg.MaxDelaySeconds > MaxDelaySecondsLimit {
		return fmt.Errorf("max_delay_seconds must be between initial_delay_seconds and %d", MaxDelaySecondsLimit)
	}
	if cfg.BackoffFactor < MinBackoffFactor || cfg.BackoffFactor > MaxBackoffFactor {
		return fmt.Errorf("backoff_factor must be between %.1f and %.1f", MinBackoffFactor, MaxBackoffFactor)
	}
	return nil
}

// ValidateCronExpression validates a cron expression string
func ValidateCronExpression(expr string) error {
	if expr == "" {
		return fmt.Errorf("cron_expression is required when trigger_type is scheduled")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron expression: %s", err.Error())
	}
	return nil
}
