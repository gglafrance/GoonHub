package ffmpeg

import "testing"

func TestCalculateTileDimensions(t *testing.T) {
	tests := []struct {
		name          string
		videoWidth    int
		videoHeight   int
		maxDimension  int
		wantTileW     int
		wantTileH     int
	}{
		{
			name:         "landscape 16:9",
			videoWidth:   1920,
			videoHeight:  1080,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    180,
		},
		{
			name:         "portrait 9:16",
			videoWidth:   1080,
			videoHeight:  1920,
			maxDimension: 320,
			wantTileW:    180,
			wantTileH:    320,
		},
		{
			name:         "square 1:1",
			videoWidth:   1080,
			videoHeight:  1080,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    320,
		},
		{
			name:         "landscape 4:3",
			videoWidth:   1440,
			videoHeight:  1080,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    240,
		},
		{
			name:         "portrait 3:4",
			videoWidth:   1080,
			videoHeight:  1440,
			maxDimension: 320,
			wantTileW:    240,
			wantTileH:    320,
		},
		{
			name:         "ultrawide 21:9",
			videoWidth:   2560,
			videoHeight:  1080,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    136,
		},
		{
			name:         "zero width defaults to max",
			videoWidth:   0,
			videoHeight:  1080,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    320,
		},
		{
			name:         "zero height defaults to max",
			videoWidth:   1920,
			videoHeight:  0,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    320,
		},
		{
			name:         "negative values defaults to max",
			videoWidth:   -1,
			videoHeight:  -1,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    320,
		},
		{
			name:         "odd ratio rounds to even",
			videoWidth:   1920,
			videoHeight:  1079,
			maxDimension: 320,
			wantTileW:    320,
			wantTileH:    180,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := CalculateTileDimensions(tt.videoWidth, tt.videoHeight, tt.maxDimension)
			if gotW != tt.wantTileW || gotH != tt.wantTileH {
				t.Errorf("CalculateTileDimensions(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.videoWidth, tt.videoHeight, tt.maxDimension,
					gotW, gotH, tt.wantTileW, tt.wantTileH)
			}
		})
	}
}

func TestRoundToEven(t *testing.T) {
	tests := []struct {
		input float64
		want  int
	}{
		{180.0, 180},
		{179.5, 180},
		{179.0, 180},
		{181.0, 182},
		{2.5, 4},
		{1.0, 2},
	}

	for _, tt := range tests {
		got := roundToEven(tt.input)
		if got != tt.want {
			t.Errorf("roundToEven(%f) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
