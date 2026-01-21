package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type VideoMetadata struct {
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func GetMetadata(videoPath string) (*VideoMetadata, error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	}

	cmd := exec.Command(FFprobePath(), args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var width, height int
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			width = stream.Width
			height = stream.Height
			break
		}
	}

	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	return &VideoMetadata{
		Duration: duration,
		Width:    width,
		Height:   height,
	}, nil
}
