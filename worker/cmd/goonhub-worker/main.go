// Command goonhub-worker processes the heavy jobs of a GoonHub server (sprites, animated
// thumbnails, fingerprints) on another machine. See README.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"goonhub-worker/internal/accel"
	"goonhub-worker/internal/config"
	"goonhub-worker/internal/logging"
	"goonhub-worker/internal/procjob"
	"goonhub-worker/internal/tools"
	"goonhub-worker/internal/worker"
)

// Set with -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = ""
	date    = ""
)

const usage = `goonhub-worker: remote job worker for GoonHub

Usage:
  goonhub-worker run      [--config FILE]   run in the foreground (Ctrl+C: graceful stop, twice: exit now)
  goonhub-worker service  install|uninstall|start|stop|restart|status [--config FILE] [--user] [--username NAME]
  goonhub-worker detect   [--config FILE]   show ffmpeg / ffprobe / fpcalc and hardware acceleration
  goonhub-worker bench    [--config FILE]   time the computations on a generated sample clip
  goonhub-worker version

The config file defaults to $GOONHUB_WORKER_CONFIG, then config.yaml next to the binary,
then the user config directory (goonhub-worker/config.yaml). See config.example.yaml.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "run":
		err = cmdRun(args)
	case "service":
		err = cmdService(args)
	case "detect":
		err = cmdDetect(args)
	case "bench":
		err = cmdBench(args)
	case "version", "--version", "-v":
		fmt.Println(versionString())
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func versionString() string {
	s := "goonhub-worker " + version
	if commit != "" {
		s += " (" + commit
		if date != "" {
			s += ", " + date
		}
		s += ")"
	}
	return s + " " + runtime.GOOS + "/" + runtime.GOARCH + " " + runtime.Version()
}

// defaultConfigPath resolves the config file when --config is not given.
func defaultConfigPath() string {
	if p := os.Getenv("GOONHUB_WORKER_CONFIG"); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "config.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "goonhub-worker", "config.yaml")
	}
	return "config.yaml"
}

func configFlag(fs *flag.FlagSet) *string {
	return fs.String("config", "", "config file (YAML)")
}

func loadConfig(path string) (*config.Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}
	return config.Load(path)
}

// setup loads the config, the logger, the tools and the acceleration.
func setup(path string, console io.Writer) (*config.Config, *slog.Logger, io.Closer, *tools.Tools, accel.Selection, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return nil, nil, nil, nil, accel.Selection{}, err
	}
	log, closer, err := logging.New(logging.Options{
		Level: cfg.Log.Level, Console: console, ConsoleFmt: cfg.Log.Format,
		File: cfg.Log.File, FileFmt: cfg.Log.FileFormat, MaxSizeMB: cfg.Log.MaxSizeMB, MaxBackups: cfg.Log.MaxBackups,
	})
	if err != nil {
		return nil, nil, nil, nil, accel.Selection{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t, err := tools.Find(ctx, tools.Configured{FFmpeg: cfg.Tools.FFmpeg, FFprobe: cfg.Tools.FFprobe, Fpcalc: cfg.Tools.Fpcalc})
	if err != nil {
		log.Error("cannot start: ffmpeg/ffprobe missing", "error", err)
		closer.Close()
		return nil, nil, nil, nil, accel.Selection{}, err
	}
	if t.Fpcalc == "" {
		log.Warn("fpcalc (chromaprint) not found: audio fingerprints disabled; the server will only send fingerprint jobs of scenes without audio. " + tools.FpcalcHelp())
	}
	sel := accel.Select(ctx, t, accel.Mode(cfg.Accel), accel.Options{VAAPIDevice: cfg.VAAPIDevice})
	log.Info("tools", "ffmpeg", t.FFmpeg, "ffmpeg_version", t.FFmpegVersion, "ffprobe", t.FFprobe, "fpcalc", t.Fpcalc, "accel", sel.Accel.Label(), "accel_reason", sel.Reason)
	return cfg, log, closer, t, sel, nil
}

func newWorker(cfg *config.Config, log *slog.Logger, t *tools.Tools, sel accel.Selection) (*worker.Worker, error) {
	// a phase this ffmpeg cannot produce is not advertised (its jobs would all fail)
	for _, p := range cfg.EnabledPhases() {
		if why := t.MissingForPhase(p); why != "" {
			log.Warn("phase disabled: "+why+": "+tools.FFmpegHelp(), "phase", p)
			cfg.Phases[p] = 0
		}
	}
	if len(cfg.EnabledPhases()) == 0 {
		return nil, fmt.Errorf("no phase this ffmpeg can produce: %s", tools.FFmpegHelp())
	}
	// a killed worker must not leave its ffmpeg running (Windows: Job Object)
	if err := procjob.KillChildrenOnExit(); err != nil {
		log.Warn("child processes may outlive a killed worker", "error", err)
	}
	return worker.New(worker.Options{Config: cfg, Tools: t, Accel: sel.Accel, Logger: log, Version: version})
}

// cmdRun runs the worker in the foreground, or under the service manager when started
// by it.
func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := configFlag(fs)
	_ = fs.Parse(args)
	if !service.Interactive() {
		return runAsService(*cfgPath)
	}
	cfg, log, closer, t, sel, err := setup(*cfgPath, os.Stderr)
	if err != nil {
		return err
	}
	defer closer.Close()
	w, err := newWorker(cfg, log, t, sel)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sig
		log.Info("stop requested: releasing running jobs (press Ctrl+C again to exit at once)", "signal", s.String())
		cancel()
		s = <-sig
		log.Warn("second stop signal: exiting now (the server requeues the jobs when their leases expire)", "signal", s.String())
		closer.Close()
		os.Exit(130)
	}()
	return w.Run(ctx)
}
