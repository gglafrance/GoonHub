package validators

import "testing"

func TestValidatePhase(t *testing.T) {
	tests := []struct {
		name    string
		phase   string
		wantErr bool
	}{
		{"valid metadata", "metadata", false},
		{"valid thumbnail", "thumbnail", false},
		{"valid sprites", "sprites", false},
		{"valid animated_thumbnails", "animated_thumbnails", false},
		{"valid scan", "scan", false},
		{"invalid phase", "invalid", true},
		{"empty phase", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhase(tt.phase)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePhase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProcessingPhase(t *testing.T) {
	tests := []struct {
		name    string
		phase   string
		wantErr bool
	}{
		{"valid metadata", "metadata", false},
		{"valid thumbnail", "thumbnail", false},
		{"valid sprites", "sprites", false},
		{"valid animated_thumbnails", "animated_thumbnails", false},
		{"scan is invalid for processing", "scan", true},
		{"invalid phase", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProcessingPhase(tt.phase)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProcessingPhase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTriggerType(t *testing.T) {
	tests := []struct {
		name        string
		phase       string
		triggerType string
		wantErr     bool
	}{
		{"on_import for metadata", "metadata", "on_import", false},
		{"after_job for thumbnail", "thumbnail", "after_job", false},
		{"manual for sprites", "sprites", "manual", false},
		{"scheduled for metadata", "metadata", "scheduled", false},
		{"manual for scan", "scan", "manual", false},
		{"scheduled for scan", "scan", "scheduled", false},
		{"on_import invalid for scan", "scan", "on_import", true},
		{"after_job invalid for scan", "scan", "after_job", true},
		{"invalid trigger type", "metadata", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTriggerType(tt.phase, tt.triggerType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTriggerType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOnImportTrigger(t *testing.T) {
	tests := []struct {
		name        string
		phase       string
		triggerType string
		wantErr     bool
	}{
		{"on_import with metadata", "metadata", "on_import", false},
		{"on_import with thumbnail", "thumbnail", "on_import", true},
		{"on_import with sprites", "sprites", "on_import", true},
		{"after_job with any phase", "thumbnail", "after_job", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOnImportTrigger(tt.phase, tt.triggerType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOnImportTrigger() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAfterJobTrigger(t *testing.T) {
	metadata := "metadata"
	thumbnail := "thumbnail"
	sprites := "sprites"
	scan := "scan"
	empty := ""

	tests := []struct {
		name       string
		phase      string
		afterPhase *string
		wantErr    bool
	}{
		{"valid after_phase metadata", "thumbnail", &metadata, false},
		{"valid after_phase thumbnail", "sprites", &thumbnail, false},
		{"valid after_phase sprites", "metadata", &sprites, false},
		{"nil after_phase", "thumbnail", nil, true},
		{"empty after_phase", "thumbnail", &empty, true},
		{"same phase", "metadata", &metadata, true},
		{"scan is invalid after_phase", "thumbnail", &scan, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAfterJobTrigger(tt.phase, tt.afterPhase)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAfterJobTrigger() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJobMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"valid missing", "missing", false},
		{"valid all", "all", false},
		{"invalid mode", "some", true},
		{"empty mode", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJobMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJobMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorkerCount(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		fieldName string
		wantErr   bool
	}{
		{"minimum valid", 1, "test_workers", false},
		{"maximum valid", 10, "test_workers", false},
		{"middle value", 5, "test_workers", false},
		{"below minimum", 0, "test_workers", true},
		{"above maximum", 11, "test_workers", true},
		{"negative", -1, "test_workers", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkerCount(tt.count, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkerCount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePoolConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PoolConfigInput
		wantErr bool
	}{
		{"all valid", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 5, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 5, FingerprintWorkers: 5}, false},
		{"minimum all", PoolConfigInput{MetadataWorkers: 1, ThumbnailWorkers: 1, SpritesWorkers: 1, AnimatedThumbnailsWorkers: 1, FingerprintWorkers: 1}, false},
		{"maximum all", PoolConfigInput{MetadataWorkers: 10, ThumbnailWorkers: 10, SpritesWorkers: 10, AnimatedThumbnailsWorkers: 10, FingerprintWorkers: 10}, false},
		{"metadata too low", PoolConfigInput{MetadataWorkers: 0, ThumbnailWorkers: 5, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 5, FingerprintWorkers: 5}, true},
		{"thumbnail too high", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 11, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 5, FingerprintWorkers: 5}, true},
		{"sprites invalid", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 5, SpritesWorkers: -1, AnimatedThumbnailsWorkers: 5, FingerprintWorkers: 5}, true},
		{"animated_thumbnails too low", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 5, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 0, FingerprintWorkers: 5}, true},
		{"animated_thumbnails too high", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 5, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 11, FingerprintWorkers: 5}, true},
		{"fingerprint too low", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 5, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 5, FingerprintWorkers: 0}, true},
		{"fingerprint too high", PoolConfigInput{MetadataWorkers: 5, ThumbnailWorkers: 5, SpritesWorkers: 5, AnimatedThumbnailsWorkers: 5, FingerprintWorkers: 11}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePoolConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePoolConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRetryConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RetryConfigInput
		wantErr bool
	}{
		{
			"all valid",
			RetryConfigInput{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 60, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
			false,
		},
		{
			"invalid phase",
			RetryConfigInput{Phase: "invalid", MaxRetries: 3, InitialDelaySeconds: 60, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
			true,
		},
		{
			"max_retries too high",
			RetryConfigInput{Phase: "metadata", MaxRetries: 11, InitialDelaySeconds: 60, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
			true,
		},
		{
			"max_retries negative",
			RetryConfigInput{Phase: "metadata", MaxRetries: -1, InitialDelaySeconds: 60, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
			true,
		},
		{
			"initial_delay too low",
			RetryConfigInput{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 0, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
			true,
		},
		{
			"max_delay less than initial",
			RetryConfigInput{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 100, MaxDelaySeconds: 50, BackoffFactor: 2.0},
			true,
		},
		{
			"backoff too low",
			RetryConfigInput{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 60, MaxDelaySeconds: 3600, BackoffFactor: 0.5},
			true,
		},
		{
			"backoff too high",
			RetryConfigInput{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 60, MaxDelaySeconds: 3600, BackoffFactor: 6.0},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRetryConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRetryConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCronExpression(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"valid every minute", "* * * * *", false},
		{"valid daily at midnight", "0 0 * * *", false},
		{"valid every hour", "0 * * * *", false},
		{"valid weekly on sunday", "0 0 * * 0", false},
		{"empty expression", "", true},
		{"invalid expression", "not a cron", true},
		{"invalid too many fields", "* * * * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCronExpression(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCronExpression() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectTriggerCycle(t *testing.T) {
	metadata := "metadata"
	thumbnail := "thumbnail"
	sprites := "sprites"

	tests := []struct {
		name          string
		configs       []TriggerConfig
		newPhase      string
		newAfterPhase string
		wantErr       bool
	}{
		{
			"no cycle - simple chain",
			[]TriggerConfig{
				{Phase: "thumbnail", TriggerType: "after_job", AfterPhase: &metadata},
			},
			"sprites",
			"thumbnail",
			false,
		},
		{
			"no cycle - independent",
			[]TriggerConfig{},
			"thumbnail",
			"metadata",
			false,
		},
		{
			"direct cycle",
			[]TriggerConfig{
				{Phase: "thumbnail", TriggerType: "after_job", AfterPhase: &metadata},
			},
			"metadata",
			"thumbnail",
			true,
		},
		{
			"indirect cycle",
			[]TriggerConfig{
				{Phase: "thumbnail", TriggerType: "after_job", AfterPhase: &metadata},
				{Phase: "sprites", TriggerType: "after_job", AfterPhase: &thumbnail},
			},
			"metadata",
			sprites,
			true,
		},
		{
			"self cycle",
			[]TriggerConfig{},
			"metadata",
			"metadata",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DetectTriggerCycle(tt.configs, tt.newPhase, tt.newAfterPhase)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectTriggerCycle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
