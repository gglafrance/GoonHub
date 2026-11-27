package jobs

import (
	"testing"
	"time"
)

func TestFingerprintJobTimeout_ScalesWithDuration(t *testing.T) {
	base := 10 * time.Minute
	tests := []struct {
		duration int
		want     time.Duration
	}{
		{0, base},                // unknown duration: pool timeout
		{600, base},              // 10 min video: pool timeout
		{3600, 30 * time.Minute}, // 1 h video: 30 min
		{10800, 90 * time.Minute},
	}
	for _, tt := range tests {
		job := &FingerprintJob{}
		job.SetSceneDuration(tt.duration)
		if got := job.JobTimeout(base); got != tt.want {
			t.Fatalf("duration %d: got %v, want %v", tt.duration, got, tt.want)
		}
	}
	var _ TimeoutScaler = (*FingerprintJob)(nil)
}
