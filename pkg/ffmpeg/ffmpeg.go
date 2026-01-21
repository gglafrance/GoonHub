package ffmpeg

import (
	"os/exec"
	"runtime"
)

const (
	ffmpegBin  = "ffmpeg"
	ffprobeBin = "ffprobe"
)

func CheckInstallation() error {
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		return err
	}
	if _, err := exec.LookPath(ffprobeBin); err != nil {
		return err
	}
	return nil
}

func FFMpegPath() string {
	return ffmpegBin
}

func FFprobePath() string {
	return ffprobeBin
}

func GetDefaultArgs() []string {
	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "-nostdin")
	}
	return args
}
