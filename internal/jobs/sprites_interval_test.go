package jobs

import "testing"

func TestEffectiveSpriteInterval(t *testing.T) {
	tests := []struct {
		name                          string
		duration, interval, minFrames int
		want                          int
	}{
		{"long scene keeps interval", 3600, 10, 12, 10},
		{"exactly enough frames", 120, 10, 12, 10},
		{"short scene gets denser frames", 60, 10, 12, 5},
		{"very short scene uses 1s", 7, 10, 12, 1},
		{"unknown duration keeps interval", 0, 10, 12, 10},
		{"disabled interval untouched", 30, 0, 12, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveSpriteInterval(tt.duration, tt.interval, tt.minFrames); got != tt.want {
				t.Fatalf("effectiveSpriteInterval(%d, %d, %d) = %d, want %d", tt.duration, tt.interval, tt.minFrames, got, tt.want)
			}
		})
	}
}
