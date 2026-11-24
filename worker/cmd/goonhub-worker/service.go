package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kardianos/service"

	"goonhub-worker/internal/worker"
)

const serviceName = "goonhub-worker"

// program adapts the worker to the service manager (Windows SCM, launchd, systemd).
type program struct {
	cfgPath string

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	w      *worker.Worker
	grace  time.Duration
	log    *slog.Logger
	closer io.Closer
}

func (p *program) Start(s service.Service) error {
	cfg, log, closer, t, sel, err := setup(p.cfgPath, os.Stderr)
	if err != nil {
		return err
	}
	w, err := newWorker(cfg, log, t, sel)
	if err != nil {
		closer.Close()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.cancel, p.done, p.w, p.grace, p.log, p.closer = cancel, make(chan struct{}), w, cfg.ShutdownGrace.D(), log, closer
	p.mu.Unlock()
	go func() {
		defer close(p.done)
		if err := w.Run(ctx); err != nil {
			log.Error("worker stopped with an error", "error", err)
			// Exit non-zero so the service manager restarts us.
			closer.Close()
			os.Exit(1)
		}
	}()
	return nil
}

// Stop is the service stop: graceful (release the jobs), bounded so the service manager
// does not kill us first.
func (p *program) Stop(s service.Service) error {
	p.mu.Lock()
	cancel, done, w, grace, log, closer := p.cancel, p.done, p.w, p.grace, p.log, p.closer
	p.mu.Unlock()
	if cancel == nil {
		return nil
	}
	log.Info("service stop: releasing running jobs")
	cancel()
	select {
	case <-done:
	case <-time.After(grace + 20*time.Second):
		log.Warn("graceful stop took too long: exiting (leases expire on the server)")
		w.Kill()
		<-done
	}
	closer.Close()
	return nil
}

func serviceConfig(cfgPath string, user bool, username string) (*service.Config, error) {
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	abs, err := filepath.Abs(cfgPath)
	if err != nil {
		return nil, err
	}
	sc := &service.Config{
		Name:        serviceName,
		DisplayName: "GoonHub worker",
		Description: "Processes GoonHub jobs (sprites, animated thumbnails, fingerprints) for a remote GoonHub server.",
		Arguments:   []string{"run", "--config", abs},
		UserName:    username,
		Option: service.KeyValue{
			// Windows: restart on failure, delayed start after boot (network up).
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "10s",
			"DelayedAutoStart":       true,
			// launchd
			"KeepAlive": true,
			"RunAtLoad": true,
			// systemd
			"Restart":     "on-failure",
			"UserService": user,
		},
		WorkingDirectory: filepath.Dir(abs),
	}
	if runtime.GOOS == "linux" {
		sc.Dependencies = []string{"After=network-online.target tailscaled.service", "Wants=network-online.target"}
	}
	return sc, nil
}

func runAsService(cfgPath string) error {
	sc, err := serviceConfig(cfgPath, false, "")
	if err != nil {
		return err
	}
	prg := &program{cfgPath: cfgPath}
	s, err := service.New(prg, sc)
	if err != nil {
		return err
	}
	return s.Run()
}

func cmdService(args []string) error {
	if len(args) == 0 {
		return errors.New("service: give install, uninstall, start, stop, restart or status")
	}
	action := args[0]
	fs := flag.NewFlagSet("service", flag.ExitOnError)
	cfgPath := configFlag(fs)
	user := fs.Bool("user", false, "macOS/Linux: install as a service of the current user (launchd agent / systemd --user)")
	username := fs.String("username", "", "run the service as this account")
	_ = fs.Parse(args[1:])

	if action == "install" {
		// Fail early on a broken config rather than at the first service start.
		if _, err := loadConfig(*cfgPath); err != nil {
			return err
		}
	}
	sc, err := serviceConfig(*cfgPath, *user, *username)
	if err != nil {
		return err
	}
	s, err := service.New(&program{cfgPath: *cfgPath}, sc)
	if err != nil {
		return err
	}
	switch action {
	case "status":
		st, err := s.Status()
		if err != nil {
			return err
		}
		switch st {
		case service.StatusRunning:
			fmt.Println("running")
		case service.StatusStopped:
			fmt.Println("stopped")
		default:
			fmt.Println("unknown")
		}
		return nil
	case "install", "uninstall", "start", "stop", "restart":
		if err := service.Control(s, action); err != nil {
			return fmt.Errorf("service %s: %w (run as administrator / root, or use --user)", action, err)
		}
		fmt.Printf("service %s: done (%s, config %s)\n", action, service.Platform(), sc.Arguments[2])
		return nil
	}
	return fmt.Errorf("service: unknown action %q", action)
}
