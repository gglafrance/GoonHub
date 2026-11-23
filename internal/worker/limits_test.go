package worker

import (
	"io"
	"log/slog"
	"testing"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/config"
)

func TestServerJobLimit(t *testing.T) {
	w := &Worker{
		cfg: &config.Config{MaxConcurrency: 3, Phases: map[workerproto.Phase]int{
			workerproto.PhaseSprites: 2, workerproto.PhaseAnimatedThumbnails: 2, workerproto.PhaseFingerprint: 0,
		}},
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		computing: map[workerproto.Phase]int{},
	}
	check := func(total, sprites, fingerprint int) {
		t.Helper()
		if got := w.totalLimitLocked(); got != total {
			t.Errorf("total = %d, want %d", got, total)
		}
		if got := w.phaseLimitLocked(workerproto.PhaseSprites); got != sprites {
			t.Errorf("sprites = %d, want %d", got, sprites)
		}
		if got := w.phaseLimitLocked(workerproto.PhaseFingerprint); got != fingerprint {
			t.Errorf("fingerprint = %d, want %d (disabled in the config)", got, fingerprint)
		}
	}

	check(3, 2, 0) // config limits

	w.setServerMaxLocked(6) // the admin raises the limit: it replaces the config's
	check(6, 6, 0)

	w.setServerMaxLocked(1) // the admin lowers it
	check(1, 1, 0)

	w.setServerMaxLocked(0) // back to the config
	check(3, 2, 0)
}
