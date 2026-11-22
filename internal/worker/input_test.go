package worker

import (
	"os"
	"path/filepath"
	"testing"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/config"
)

func testWorker(t *testing.T, storage []config.StorageMapping) *Worker {
	t.Helper()
	return &Worker{cfg: &config.Config{Storage: storage, MaxConcurrency: 2, MaxPendingUploads: 4,
		Phases: map[workerproto.Phase]int{workerproto.PhaseSprites: 1, workerproto.PhaseFingerprint: 2}},
		computing: map[workerproto.Phase]int{}}
}

func TestDirectPath(t *testing.T) {
	mount := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mount, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(mount, "a", "v.mp4")
	if err := os.WriteFile(video, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	id := uint(2)
	other := uint(3)
	w := testWorker(t, []config.StorageMapping{
		{StoragePathID: 2, LocalPath: mount},
		{ServerPrefix: "/media/stash", LocalPath: mount},
	})
	cases := []struct {
		name string
		in   workerproto.InputSpec
		ok   bool
	}{
		{"by id", workerproto.InputSpec{StoragePathID: &id, RelativePath: "a/v.mp4", Size: 100}, true},
		{"by prefix", workerproto.InputSpec{ServerPath: "/media/stash/a/v.mp4", Size: 100}, true},
		{"size differs", workerproto.InputSpec{StoragePathID: &id, RelativePath: "a/v.mp4", Size: 99}, false},
		{"other storage", workerproto.InputSpec{StoragePathID: &other, RelativePath: "a/v.mp4", Size: 100}, false},
		{"traversal by id", workerproto.InputSpec{StoragePathID: &id, RelativePath: "../outside.mp4", Size: 100}, false},
		{"traversal by prefix", workerproto.InputSpec{ServerPath: "/media/stash/../etc/passwd", Size: 100}, false},
		{"prefix lookalike", workerproto.InputSpec{ServerPath: "/media/stashed/a/v.mp4", Size: 100}, false},
		{"directory", workerproto.InputSpec{StoragePathID: &id, RelativePath: "a", Size: 0}, false},
		{"missing", workerproto.InputSpec{StoragePathID: &id, RelativePath: "a/none.mp4", Size: 100}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ok, why := w.directPath(&workerproto.Assignment{Input: c.in})
			if ok != c.ok {
				t.Fatalf("ok %v (%q, %s), want %v", ok, p, why, c.ok)
			}
			if ok && p != video {
				t.Fatalf("path %q, want %q", p, video)
			}
		})
	}
}

func TestFreeSlots(t *testing.T) {
	w := testWorker(t, nil)
	w.sess.id = "s"
	slots := w.freeSlotsLocked()
	if slots[workerproto.PhaseSprites] != 1 || slots[workerproto.PhaseFingerprint] != 2 {
		t.Fatalf("idle slots %v", slots)
	}
	w.computing[workerproto.PhaseFingerprint] = 1
	w.computingTotal = 1
	slots = w.freeSlotsLocked()
	if slots[workerproto.PhaseSprites] != 1 || slots[workerproto.PhaseFingerprint] != 1 {
		t.Fatalf("slots %v (global cap 1 left)", slots)
	}
	w.computingTotal = 2
	if s := w.freeSlotsLocked(); len(s) != 0 {
		t.Fatalf("full worker claims %v", s)
	}
	w.computingTotal = 0
	w.drain = true
	if s := w.freeSlotsLocked(); len(s) != 0 {
		t.Fatalf("draining worker claims %v", s)
	}
	w.drain = false
	w.diskLow = true
	if s := w.freeSlotsLocked(); len(s) != 0 {
		t.Fatalf("disk-low worker claims %v", s)
	}
	w.diskLow = false
	w.delivering = 4
	if s := w.freeSlotsLocked(); len(s) != 0 {
		t.Fatalf("worker with full upload queue claims %v", s)
	}
}

func TestPhaseTimeout(t *testing.T) {
	w := testWorker(t, nil)
	w.cfg.Timeouts.Fingerprint = config.Duration(5 * 60e9)
	a := &workerproto.Assignment{Phase: workerproto.PhaseFingerprint, Input: workerproto.InputSpec{Video: workerproto.VideoInfo{DurationSec: 7200}}}
	if got := w.phaseTimeout(a); got.Minutes() != 60 {
		t.Fatalf("fingerprint timeout for 2 h: %v, want 1h (0.5 s per second)", got)
	}
	a.Input.Video.DurationSec = 60
	if got := w.phaseTimeout(a); got.Minutes() != 5 {
		t.Fatalf("fingerprint timeout for 1 min: %v, want 5m", got)
	}
}
