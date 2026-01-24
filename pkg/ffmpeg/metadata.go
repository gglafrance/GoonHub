package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type VideoMetadata struct {
	Duration   float64 `json:"duration"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FrameRate  float64 `json:"frame_rate"`
	BitRate    int64   `json:"bit_rate"`
	VideoCodec string  `json:"video_codec"`
	AudioCodec string  `json:"audio_codec"`
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		RFrameRate   string `json:"r_frame_rate"`
		AvgFrameRate string `json:"avg_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
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
	var videoCodec, audioCodec string
	var frameRate float64
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" && width == 0 {
			width = stream.Width
			height = stream.Height
			videoCodec = stream.CodecName
			frameRate = parseFrameRate(stream.RFrameRate)
		}
		if stream.CodecType == "audio" && audioCodec == "" {
			audioCodec = stream.CodecName
		}
	}

	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	var bitRate int64
	if probe.Format.BitRate != "" {
		bitRate, _ = strconv.ParseInt(probe.Format.BitRate, 10, 64)
	}

	return &VideoMetadata{
		Duration:   duration,
		Width:      width,
		Height:     height,
		FrameRate:  frameRate,
		BitRate:    bitRate,
		VideoCodec: videoCodec,
		AudioCodec: audioCodec,
	}, nil
}

func parseFrameRate(rate string) float64 {
	if rate == "" {
		return 0
	}
	parts := strings.SplitN(rate, "/", 2)
	if len(parts) != 2 {
		val, _ := strconv.ParseFloat(rate, 64)
		return val
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || den == 0 {
		return 0
	}
	return num / den
}
