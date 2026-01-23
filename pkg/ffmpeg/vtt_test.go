package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{0, "00:00:00.000"},
		{59, "00:00:59.000"},
		{60, "00:01:00.000"},
		{3599, "00:59:59.000"},
		{3600, "01:00:00.000"},
		{86399, "23:59:59.000"},
		{90061, "25:01:01.000"}, // >24h: hours overflow, minutes/seconds still computed modularly
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatTime(tt.seconds)
			if result != tt.expected {
				t.Fatalf("formatTime(%d) = %q, want %q", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestGenerateVtt_SingleSheet(t *testing.T) {
	dir := t.TempDir()
	vttPath := filepath.Join(dir, "test.vtt")

	spriteSheets := []string{"1_sheet_0.jpg"}
	// 60s video, 5s interval, 4x4 grid, 160x90 tiles
	err := GenerateVttFile(vttPath, spriteSheets, 60, 5, 4, 4, 160, 90)
	if err != nil {
		t.Fatalf("GenerateVttFile failed: %v", err)
	}

	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read VTT file: %v", err)
	}

	vtt := string(content)
	if !strings.HasPrefix(vtt, "WEBVTT") {
		t.Fatal("VTT file should start with WEBVTT header")
	}

	// 60s / 5s = 12 frames
	cueCount := strings.Count(vtt, "-->")
	if cueCount != 12 {
		t.Fatalf("expected 12 cues for 60s video with 5s interval, got %d", cueCount)
	}

	// All should reference sheet_0
	if !strings.Contains(vtt, "/sprites/1_sheet_0.jpg") {
		t.Fatal("expected sprite sheet reference in VTT")
	}
}

func TestGenerateVtt_MultipleSheets(t *testing.T) {
	dir := t.TempDir()
	vttPath := filepath.Join(dir, "test.vtt")

	spriteSheets := []string{"1_sheet_0.jpg", "1_sheet_1.jpg"}
	// 600s video, 5s interval, 4x4 grid (16 per sheet) = 120 frames, needs 8 sheets
	// but only 2 provided, so output should stop at 32 frames
	err := GenerateVttFile(vttPath, spriteSheets, 600, 5, 4, 4, 160, 90)
	if err != nil {
		t.Fatalf("GenerateVttFile failed: %v", err)
	}

	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read VTT file: %v", err)
	}

	vtt := string(content)
	cueCount := strings.Count(vtt, "-->")
	// 2 sheets * 16 frames per sheet = 32 frames max
	if cueCount != 32 {
		t.Fatalf("expected 32 cues (limited by available sheets), got %d", cueCount)
	}

	// Verify both sheets are referenced
	if !strings.Contains(vtt, "/sprites/1_sheet_0.jpg") {
		t.Fatal("expected first sprite sheet reference")
	}
	if !strings.Contains(vtt, "/sprites/1_sheet_1.jpg") {
		t.Fatal("expected second sprite sheet reference")
	}
}

func TestGenerateVtt_PartialLastGrid(t *testing.T) {
	dir := t.TempDir()
	vttPath := filepath.Join(dir, "test.vtt")

	spriteSheets := []string{"1_sheet_0.jpg"}
	// 65s video, 5s interval, 4x4 grid = 13 frames (doesn't fill the 16-tile grid)
	err := GenerateVttFile(vttPath, spriteSheets, 65, 5, 4, 4, 160, 90)
	if err != nil {
		t.Fatalf("GenerateVttFile failed: %v", err)
	}

	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read VTT file: %v", err)
	}

	vtt := string(content)
	cueCount := strings.Count(vtt, "-->")
	if cueCount != 13 {
		t.Fatalf("expected 13 cues for 65s/5s partial grid, got %d", cueCount)
	}
}

func TestGenerateVtt_CoordinateCalculation(t *testing.T) {
	dir := t.TempDir()
	vttPath := filepath.Join(dir, "test.vtt")

	spriteSheets := []string{"1_sheet_0.jpg"}
	// 4x4 grid, 160x90 tiles, 5s interval
	err := GenerateVttFile(vttPath, spriteSheets, 80, 5, 4, 4, 160, 90)
	if err != nil {
		t.Fatalf("GenerateVttFile failed: %v", err)
	}

	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read VTT file: %v", err)
	}

	vtt := string(content)

	// Frame 0: col=0, row=0 -> xywh=0,0,160,90
	if !strings.Contains(vtt, "#xywh=0,0,160,90") {
		t.Fatal("frame 0 should be at 0,0")
	}

	// Frame 1: col=1, row=0 -> xywh=160,0,160,90
	if !strings.Contains(vtt, "#xywh=160,0,160,90") {
		t.Fatal("frame 1 should be at 160,0")
	}

	// Frame 4: col=0, row=1 -> xywh=0,90,160,90
	if !strings.Contains(vtt, "#xywh=0,90,160,90") {
		t.Fatal("frame 4 should be at 0,90")
	}

	// Frame 5: col=1, row=1 -> xywh=160,90,160,90
	if !strings.Contains(vtt, "#xywh=160,90,160,90") {
		t.Fatal("frame 5 should be at 160,90")
	}

	// Frame 15: col=3, row=3 -> xywh=480,270,160,90
	if !strings.Contains(vtt, "#xywh=480,270,160,90") {
		t.Fatal("frame 15 (last in grid) should be at 480,270")
	}
}

func TestGenerateVtt_FirstFrameAt0_0(t *testing.T) {
	dir := t.TempDir()
	vttPath := filepath.Join(dir, "test.vtt")

	spriteSheets := []string{"1_sheet_0.jpg"}
	err := GenerateVttFile(vttPath, spriteSheets, 10, 5, 4, 4, 200, 100)
	if err != nil {
		t.Fatalf("GenerateVttFile failed: %v", err)
	}

	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("failed to read VTT file: %v", err)
	}

	vtt := string(content)

	// First cue should start at 00:00:00 and position at origin
	lines := strings.Split(vtt, "\n")
	foundFirst := false
	for i, line := range lines {
		if strings.Contains(line, "00:00:00.000 --> 00:00:05.000") {
			if i+1 < len(lines) && strings.Contains(lines[i+1], "#xywh=0,0,200,100") {
				foundFirst = true
				break
			}
		}
	}
	if !foundFirst {
		t.Fatal("first frame cue should start at 00:00:00.000 with position 0,0")
	}
}
