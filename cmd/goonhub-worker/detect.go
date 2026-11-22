package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/accel"
	"goonhub-worker/internal/config"
	"goonhub-worker/internal/tools"
)

// detectTools finds the tools from the config when there is one, else with the defaults.
func detectTools(cfgPath string) (*config.Config, *tools.Tools, error) {
	cfg, cerr := loadConfig(cfgPath)
	conf := tools.Configured{}
	if cerr == nil {
		conf = tools.Configured{FFmpeg: cfg.Tools.FFmpeg, FFprobe: cfg.Tools.FFprobe, Fpcalc: cfg.Tools.Fpcalc}
	} else {
		fmt.Printf("config: not loaded (%v): using auto-detection\n\n", cerr)
		cfg = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	t, err := tools.Find(ctx, conf)
	return cfg, t, err
}

func cmdDetect(args []string) error {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	cfgPath := configFlag(fs)
	_ = fs.Parse(args)
	fmt.Println(versionString())
	fmt.Println()
	cfg, t, err := detectTools(*cfgPath)
	if err != nil {
		fmt.Println("ffmpeg :", orMissing(t.FFmpeg))
		fmt.Println("ffprobe:", orMissing(t.FFprobe))
		return err
	}
	fmt.Printf("ffmpeg : %s (version %s)\n", t.FFmpeg, t.FFmpegVersion)
	fmt.Printf("ffprobe: %s\n", t.FFprobe)
	if t.Fpcalc != "" {
		fmt.Printf("fpcalc : %s (version %s): audio fingerprints enabled\n", t.Fpcalc, t.FpcalcVersion)
	} else {
		fmt.Println("fpcalc : MISSING: audio fingerprints disabled (the server then only sends fingerprint jobs of scenes without audio)")
	}
	fmt.Printf("hwaccels (ffmpeg -hwaccels): %s\n", strings.Join(t.HWAccels, ", "))
	fmt.Printf("H.264/HEVC encoders        : %s\n", strings.Join(t.Encoders, ", "))
	mode := accel.ModeAuto
	if cfg != nil {
		mode = accel.Mode(cfg.Accel)
	}
	sel := accel.Select(context.Background(), t, mode)
	fmt.Printf("acceleration in use        : %s (%s)\n", sel.Accel.Label(), sel.Reason)
	if cfg != nil {
		fmt.Println()
		fmt.Printf("server: %s (mode %s), phases %v, max_concurrency %d\n", cfg.Server, cfg.Mode, cfg.EnabledPhases(), cfg.MaxConcurrency)
		for _, m := range cfg.Storage {
			state := "ok"
			if st, err := os.Stat(m.LocalPath); err != nil {
				state = "NOT ACCESSIBLE: " + err.Error()
			} else if !st.IsDir() {
				state = "NOT A DIRECTORY"
			}
			fmt.Printf("direct storage: id=%d prefix=%q -> %s: %s\n", m.StoragePathID, m.ServerPrefix, m.LocalPath, state)
		}
	}
	if t.Fpcalc == "" {
		fmt.Println()
		fmt.Println(tools.FpcalcHelp())
	}
	return nil
}

func orMissing(s string) string {
	if s == "" {
		return "MISSING"
	}
	return s
}

func cmdBench(args []string) error {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	cfgPath := configFlag(fs)
	seconds := fs.Int("seconds", 60, "length of the generated sample clip")
	input := fs.String("input", "", "use this video instead of a generated clip (read only)")
	_ = fs.Parse(args)
	cfg, t, err := detectTools(*cfgPath)
	if err != nil {
		return err
	}
	mode := accel.ModeAuto
	if cfg != nil {
		mode = accel.Mode(cfg.Accel)
	}
	sel := accel.Select(context.Background(), t, mode)
	dir, err := os.MkdirTemp("", "goonhub-worker-bench-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	clip := *input
	video := workerproto.VideoInfo{DurationSec: *seconds, Width: 1280, Height: 720, VideoCodec: "h264", AudioCodec: "aac"}
	if clip == "" {
		clip = filepath.Join(dir, "sample.mp4")
		fmt.Printf("generating a %d s 1280x720 sample clip...\n", *seconds)
		out, err := exec.Command(t.FFmpeg, "-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=duration=%d:size=1280x720:rate=30", *seconds),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", *seconds),
			"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", "-y", clip).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to generate the sample clip: %v: %s", err, out)
		}
	} else {
		video = workerproto.VideoInfo{DurationSec: *seconds, AudioCodec: "aac"}
		fmt.Println("note: --seconds must match the input duration for the sprites interval")
	}
	fmt.Printf("acceleration: %s (%s)\n\n", sel.Accel.Label(), sel.Reason)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	for _, r := range accel.Bench(ctx, t, sel, clip, video, dir) {
		if r.Err != nil {
			fmt.Printf("%-22s FAILED after %s: %v\n", r.Name, r.Duration.Round(time.Millisecond), r.Err)
			continue
		}
		fmt.Printf("%-22s %s\n", r.Name, r.Duration.Round(time.Millisecond))
	}
	return nil
}
