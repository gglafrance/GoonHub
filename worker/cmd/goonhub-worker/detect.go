package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"goonhub/pkg/compute/hwaccel"
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
	for _, p := range []workerproto.Phase{workerproto.PhaseSprites, workerproto.PhaseAnimatedThumbnails} {
		if why := t.MissingForPhase(p); why != "" {
			fmt.Printf("%-27s: DISABLED: %s\n%29s%s\n", p, why, "", tools.FFmpegHelp())
		}
	}
	sel := selectAccel(cfg, t)
	printReport(sel)
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

// selectAccel runs the hardware detection with the config (auto without one).
func selectAccel(cfg *config.Config, t *tools.Tools) accel.Selection {
	mode, o := accel.ModeAuto, accel.Options{}
	if cfg != nil {
		mode, o.VAAPIDevice = accel.Mode(cfg.Accel), cfg.VAAPIDevice
	}
	fmt.Println("probing the hardware pipelines (a few seconds)...")
	return accel.Select(context.Background(), t, mode, o)
}

// printReport prints what the detection proved, method by method.
func printReport(sel accel.Selection) {
	r := sel.Report
	if r == nil {
		return
	}
	if len(r.Devices) > 0 {
		fmt.Printf("GPUs                       : %s\n", strings.Join(r.Devices, "; "))
	}
	for _, mr := range r.Methods {
		if mr.Skipped != "" {
			fmt.Printf("  %-13s skipped: %s\n", mr.Method, mr.Skipped)
			continue
		}
		var ok, ko []string
		for _, c := range mr.Candidates {
			name := string(c.Step) + "/" + string(c.Variant)
			if c.OK {
				ok = append(ok, name)
			} else {
				ko = append(ko, name)
			}
		}
		line := "proven: " + orNone(ok)
		if len(ko) > 0 {
			line += "; failed: " + strings.Join(ko, ",")
		}
		fmt.Printf("  %-13s %s\n", mr.Method, line)
	}
}

func orNone(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return strings.Join(l, ",")
}

func cmdBench(args []string) error {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	cfgPath := configFlag(fs)
	seconds := fs.Int("seconds", 20, "length of the generated sample clip")
	codec := fs.String("codec", "h264", "codec of the generated sample clip: h264 or hevc")
	input := fs.String("input", "", "use this video instead of a generated clip (read only)")
	asJSON := fs.Bool("json", false, "print the full report as JSON")
	_ = fs.Parse(args)
	cfg, t, err := detectTools(*cfgPath)
	if err != nil {
		return err
	}
	sel := selectAccel(cfg, t)
	fmt.Printf("acceleration: %s (%s)\n", sel.Accel.Label(), sel.Reason)
	if *input == "" {
		fmt.Printf("generating a %d s 1920x1080 %s sample clip...\n", *seconds, *codec)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	rep, err := accel.Bench(ctx, t, sel, hwaccel.Sample{Path: *input,
		Spec: hwaccel.SampleSpec{Codec: *codec, Seconds: *seconds}})
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	fmt.Printf("\nsample %s (%s %dx%d, %.0f s)\n\n", rep.Sample, rep.Video.Codec, rep.Video.Width, rep.Video.Height, rep.Video.Duration)
	fmt.Printf("%-15s %-34s %10s %10s %8s %6s  %s\n", "step", "accelerated", "software", "accel", "speedup", "cpu", "verdict")
	for i := range rep.Steps {
		sb := &rep.Steps[i]
		sw := fmt.Sprintf("%d ms", sb.Software.WallMS)
		if sb.Accelerated == nil {
			msg := "software only"
			if sb.Error != "" {
				msg = "FAILED: " + sb.Error
			}
			fmt.Printf("%-15s %-34s %10s %10s %8s %6s  %s\n", sb.Step, orDash(sb.Accel), sw, "-", "-", "-", msg)
			continue
		}
		verdict := "not worth it (software kept)"
		if sb.Worth() {
			verdict = "worth it"
		}
		if h := sb.HashDrift; h != nil {
			verdict += fmt.Sprintf(", hashes: %d/%d exact, max distance %d", h.Exact, h.Compared, h.Max)
		}
		fmt.Printf("%-15s %-34s %10s %7d ms %7.2fx %5.0f%%  %s\n", sb.Step, sb.Accel, sw,
			sb.Accelerated.WallMS, sb.Speedup, sb.CPURatio*100, verdict)
	}
	fmt.Println("\ncpu = accelerated CPU time / software CPU time.")
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
