package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
)

func GenerateVttFile(vttPath string, spriteSheets []string, videoDuration, interval, gridCols, gridRows, width, height int) error {
	if err := os.MkdirAll(filepath.Dir(vttPath), 0755); err != nil {
		return fmt.Errorf("failed to create VTT directory: %w", err)
	}

	vttContent := "WEBVTT\n\n"

	totalFrames := videoDuration / interval
	if videoDuration%interval != 0 {
		totalFrames++
	}

	framesPerSheet := gridCols * gridRows

	for i := 0; i < totalFrames; i++ {
		startTime := i * interval
		endTime := startTime + interval

		sheetIndex := i / framesPerSheet
		if sheetIndex >= len(spriteSheets) {
			break
		}

		sheetFilename := spriteSheets[sheetIndex]
		sheetUrl := fmt.Sprintf("/sprites/%s", sheetFilename)

		frameInSheet := i % framesPerSheet
		col := frameInSheet % gridCols
		row := frameInSheet / gridCols

		x := col * width
		y := row * height

		vttContent += fmt.Sprintf("%s --> %s\n", formatTime(startTime), formatTime(endTime))
		vttContent += fmt.Sprintf("%s#xywh=%d,%d,%d,%d\n\n", sheetUrl, x, y, width, height)
	}

	if err := os.WriteFile(vttPath, []byte(vttContent), 0644); err != nil {
		return fmt.Errorf("failed to write VTT file: %w", err)
	}

	return nil
}

func formatTime(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	millis := 0
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
