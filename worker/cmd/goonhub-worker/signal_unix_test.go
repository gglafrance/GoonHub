//go:build unix

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/fakeserver"
)

// TestBinaryStopSignals builds the real binary and stops it like a terminal or a service
// manager would: SIGINT to the whole process group (ffmpeg children included, as Ctrl+C
// does) must release the job (shutdown, no attempt counted); a second signal exits at
// once.
func TestBinaryStopSignals(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "goonhub-worker")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	clip := filepath.Join(dir, "long.mp4")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
		"testsrc=duration=600:size=320x240:rate=25", "-c:v", "libx264", "-preset", "ultrafast", "-g", "25", "-y", clip).CombinedOutput(); err != nil {
		t.Fatalf("clip: %v: %s", err, out)
	}

	for _, tc := range []struct {
		name    string
		signals int
	}{{"graceful", 1}, {"twice", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeserver.New()
			defer srv.Close()
			id := srv.AddJob(workerproto.PhaseSprites, 1, clip, workerproto.VideoInfo{DurationSec: 600, Width: 320, Height: 240},
				workerproto.PhaseParams{Sprites: &workerproto.SpritesParams{SceneID: 1, TileWidth: 160, TileHeight: 120,
					FrameInterval: 1, FrameQuality: 70, GridCols: 10, GridRows: 10, Concurrency: 1}})
			wdir := t.TempDir()
			cfg := filepath.Join(wdir, "config.yaml")
			y := fmt.Sprintf("server: %s\ntoken: %s\nphases:\n  sprites: 1\nmin_free_disk: 1\nlog:\n  file: \"-\"\n", srv.URL(), srv.Token)
			if err := os.WriteFile(cfg, []byte(y), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "run", "--config", cfg)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			stderr, _ := cmd.StderrPipe()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			computing := make(chan struct{})
			var logs strings.Builder
			go func() {
				sc := bufio.NewScanner(stderr)
				once := false
				for sc.Scan() {
					logs.WriteString(sc.Text() + "\n")
					if !once && strings.Contains(sc.Text(), "msg=computing") {
						once = true
						close(computing)
					}
				}
			}()
			select {
			case <-computing:
			case <-time.After(30 * time.Second):
				_ = cmd.Process.Kill()
				t.Fatalf("worker never started computing:\n%s", logs.String())
			}
			time.Sleep(500 * time.Millisecond)
			if tc.signals == 2 {
				// Server unreachable: the graceful stop would retry its releases for
				// seconds; the second signal must not wait for that.
				srv.SetDown(true)
			}
			// Like Ctrl+C in a terminal: the whole process group, ffmpeg included.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
			if tc.signals == 2 {
				time.Sleep(300 * time.Millisecond)
				_ = syscall.Kill(cmd.Process.Pid, syscall.SIGINT)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			var err error
			select {
			case err = <-done:
			case <-time.After(15 * time.Second):
				_ = cmd.Process.Kill()
				t.Fatalf("worker did not exit:\n%s", logs.String())
			}
			j := srv.Job(id)
			if tc.signals == 1 {
				if err != nil {
					t.Fatalf("exit: %v\n%s", err, logs.String())
				}
				if j.Status != fakeserver.Pending || j.RetryCount != 0 || len(j.Releases) == 0 || j.Releases[0] != workerproto.ReleaseShutdown || j.Failure != nil {
					t.Fatalf("job after Ctrl+C: %+v\n%s", j, logs.String())
				}
				return
			}
			var ee *exec.ExitError
			if err == nil {
				t.Log("the graceful stop finished before the second signal")
			} else if !errorsAs(err, &ee) || ee.ExitCode() != 130 {
				t.Fatalf("second signal: exit %v, want 130\n%s", err, logs.String())
			}
			if j.Failure != nil {
				t.Fatalf("job failed on a stop: %+v", j.Failure)
			}
		})
	}
}

type lockedBuilder struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuilder) add(s string) {
	l.mu.Lock()
	l.b.WriteString(s)
	l.mu.Unlock()
}

func (l *lockedBuilder) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func errorsAs(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}
