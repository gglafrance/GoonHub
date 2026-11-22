// Package tools locates ffmpeg, ffprobe and fpcalc and reads what the local ffmpeg
// supports (version, hardware acceleration methods, H.264 encoders).
package tools

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Tools are the resolved binaries.
type Tools struct {
	FFmpeg  string
	FFprobe string
	Fpcalc  string // "" when not found (no audio fingerprint)

	FFmpegVersion string
	FpcalcVersion string
	HWAccels      []string // ffmpeg -hwaccels
	Encoders      []string // H.264 / HEVC encoders of ffmpeg -encoders
}

// Configured are the paths from the config ("" = auto).
type Configured struct {
	FFmpeg, FFprobe, Fpcalc string
}

// ErrMissing is wrapped by Find when a required binary is missing.
var ErrMissing = errors.New("required binary not found")

// Find resolves the binaries: the configured path, else next to the worker executable
// (also in a "bin" subdirectory), else PATH. ffmpeg and ffprobe are required; fpcalc is
// optional (without it the worker does not advertise audio fingerprints).
func Find(ctx context.Context, c Configured) (*Tools, error) {
	t := &Tools{}
	var errs []error
	var err error
	if t.FFmpeg, err = locate("ffmpeg", c.FFmpeg); err != nil {
		errs = append(errs, err)
	}
	if t.FFprobe, err = locate("ffprobe", c.FFprobe); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return t, fmt.Errorf("%w: %w\n%s", ErrMissing, errors.Join(errs...), InstallHelp())
	}
	if t.Fpcalc, err = locate("fpcalc", c.Fpcalc); err != nil {
		t.Fpcalc = ""
		if c.Fpcalc != "" {
			// An explicit path that does not exist is a config error.
			return t, err
		}
	}
	t.FFmpegVersion = probeVersion(ctx, t.FFmpeg, "-version")
	if t.FFmpegVersion == "" {
		return t, fmt.Errorf("%s does not run (ffmpeg -version failed): check the binary matches this OS/CPU", t.FFmpeg)
	}
	if t.Fpcalc != "" {
		t.FpcalcVersion = probeVersion(ctx, t.Fpcalc, "-version")
	}
	t.HWAccels = listHWAccels(ctx, t.FFmpeg)
	t.Encoders = listEncoders(ctx, t.FFmpeg)
	return t, nil
}

// HasAudioFingerprint reports whether fpcalc is available.
func (t *Tools) HasAudioFingerprint() bool { return t.Fpcalc != "" }

func exeName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}

func locate(name, configured string) (string, error) {
	if configured != "" {
		if st, err := os.Stat(configured); err == nil && !st.IsDir() {
			return configured, nil
		}
		if p, err := exec.LookPath(configured); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%s: configured path %q not found", name, configured)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{filepath.Join(dir, exeName(name)), filepath.Join(dir, "bin", exeName(name))} {
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand, nil
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found next to the worker binary nor on PATH", name)
}

func run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, bin, args...).CombinedOutput()
}

func probeVersion(ctx context.Context, bin, flag string) string {
	out, err := run(ctx, bin, flag)
	if err != nil && len(out) == 0 {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	// "ffmpeg version N-124716-g054dffd133-20251123 Copyright ..." -> "N-124716-g054dffd133-20251123"
	// "fpcalc version 1.5.1 (FFmpeg ...)" -> "1.5.1"
	if i := strings.Index(line, " version "); i >= 0 {
		rest := strings.Fields(line[i+len(" version "):])
		if len(rest) > 0 {
			return rest[0]
		}
	}
	return line
}

func listHWAccels(ctx context.Context, ffmpeg string) []string {
	out, err := run(ctx, ffmpeg, "-hide_banner", "-hwaccels")
	if err != nil {
		return nil
	}
	return ParseHWAccels(out)
}

// ParseHWAccels parses the output of "ffmpeg -hwaccels".
func ParseHWAccels(out []byte) []string {
	var res []string
	started := false
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Hardware acceleration methods") {
			started = true
			continue
		}
		if started && line != "" {
			res = append(res, line)
		}
	}
	return res
}

func listEncoders(ctx context.Context, ffmpeg string) []string {
	out, err := run(ctx, ffmpeg, "-hide_banner", "-encoders")
	if err != nil {
		return nil
	}
	return ParseEncoders(out)
}

// ParseEncoders keeps the H.264 / HEVC video encoders of "ffmpeg -encoders".
func ParseEncoders(out []byte) []string {
	var res []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 || len(f[0]) != 6 || f[0][0] != 'V' {
			continue
		}
		name := f[1]
		if name == "libx264" || name == "libx265" || strings.HasPrefix(name, "h264_") || strings.HasPrefix(name, "hevc_") {
			res = append(res, name)
		}
	}
	return res
}

// FpcalcHelp explains where to get fpcalc (chromaprint) for this OS.
func FpcalcHelp() string {
	switch runtime.GOOS {
	case "windows":
		return "fpcalc.exe: https://github.com/acoustid/chromaprint/releases (chromaprint-fpcalc-*-windows-x86_64.zip); put it next to goonhub-worker.exe, on PATH, or set tools.fpcalc."
	case "darwin":
		return "fpcalc: brew install chromaprint, or https://github.com/acoustid/chromaprint/releases (chromaprint-fpcalc-*-macos-universal.tar.gz); next to the binary, on PATH, or tools.fpcalc."
	default:
		return "fpcalc: apt install libchromaprint-tools / dnf install chromaprint-tools, or https://github.com/acoustid/chromaprint/releases (chromaprint-fpcalc-*-linux-x86_64.tar.gz); next to the binary, on PATH, or tools.fpcalc."
	}
}

// InstallHelp explains where to get the binaries for this OS.
func InstallHelp() string {
	switch runtime.GOOS {
	case "windows":
		return "Install ffmpeg: winget install Gyan.FFmpeg (or a BtbN build from https://github.com/BtbN/FFmpeg-Builds/releases, ffmpeg-master-latest-win64-gpl), " +
			"or put ffmpeg.exe and ffprobe.exe next to goonhub-worker.exe, or set tools.ffmpeg / tools.ffprobe in the config. " +
			"Audio fingerprints need fpcalc.exe (chromaprint): https://github.com/acoustid/chromaprint/releases (chromaprint-fpcalc-*-windows-x86_64.zip)."
	case "darwin":
		return "Install ffmpeg: brew install ffmpeg chromaprint (or static builds from https://evermeet.cx/ffmpeg/ / https://www.osxexperts.net/), " +
			"or put ffmpeg and ffprobe next to goonhub-worker, or set tools.ffmpeg / tools.ffprobe. " +
			"fpcalc: brew install chromaprint, or https://github.com/acoustid/chromaprint/releases (chromaprint-fpcalc-*-macos-universal.tar.gz)."
	default:
		return "Install ffmpeg: apt install ffmpeg libchromaprint-tools (Debian/Ubuntu), dnf install ffmpeg chromaprint-tools, " +
			"or static builds from https://johnvansickle.com/ffmpeg/ / https://github.com/BtbN/FFmpeg-Builds/releases; " +
			"fpcalc: https://github.com/acoustid/chromaprint/releases (chromaprint-fpcalc-*-linux-x86_64.tar.gz)."
	}
}
