package jobs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"goonhub/internal/mocks"
	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// End-to-end tests of the jobs (prepare -> pkg/compute -> commit) with real ffmpeg; skipped
// without ffmpeg on PATH.

func renderClip(t *testing.T, seconds int, withAudio bool) string {
	t.Helper()
	if err := ffmpeg.CheckInstallation(); err != nil {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	path := filepath.Join(t.TempDir(), "clip.mp4")
	args := []string{"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=25", seconds)}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds), "-c:a", "aac")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", path)
	if out, err := exec.Command(ffmpeg.FFMpegPath(), args...).CombinedOutput(); err != nil {
		t.Fatalf("failed to render clip: %v, output: %s", err, out)
	}
	return path
}

func TestSpritesJob_ComputesAndCommits(t *testing.T) {
	clip := renderClip(t, 20, false)
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSceneRepository(ctrl)
	root := t.TempDir()
	spriteDir, vttDir := filepath.Join(root, "sprites"), filepath.Join(root, "vtt")

	repo.EXPECT().UpdateSprites(uint(3), filepath.Join(spriteDir, "3_sheet_001.webp"), filepath.Join(vttDir, "3_thumbnails.vtt"), 1).Return(nil)

	job := NewSpritesJobWithID("job-sprites", 3, clip, spriteDir, vttDir, 64, 48, 20, 5, 60, 4, 2, 2, repo, zap.NewNop())
	var last atomic.Int32 // progress is reported from the frame goroutines
	job.SetProgressCallback(func(_ string, p int) {
		if p == 100 {
			last.Store(int32(p))
		}
	})
	if err := job.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if job.GetStatus() != JobStatusCompleted || job.GetResult().SpriteSheetCount != 1 || last.Load() != 100 {
		t.Fatalf("status %s result %+v progress %d", job.GetStatus(), job.GetResult(), last.Load())
	}
	if !strings.Contains(readFile(t, filepath.Join(vttDir, "3_thumbnails.vtt")), "/sprites/3_sheet_001.webp#xywh=") {
		t.Fatal("VTT not committed")
	}
	if _, err := os.Stat(LocalStagingDir(StagingRoot("", spriteDir), "job-sprites")); !os.IsNotExist(err) {
		t.Fatal("the staging directory must be removed after the job")
	}
}

func TestSpritesJob_MissingSourceFails(t *testing.T) {
	if err := ffmpeg.CheckInstallation(); err != nil {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSceneRepository(ctrl)
	repo.EXPECT().UpdateProcessingStatus(uint(3), string(JobStatusFailed), gomock.Any()).Return(nil)
	root := t.TempDir()
	job := NewSpritesJob(3, filepath.Join(root, "missing.mp4"), filepath.Join(root, "s"), filepath.Join(root, "v"), 64, 48, 20, 5, 60, 4, 2, 2, repo, zap.NewNop())
	if err := job.Execute(); err == nil || job.GetStatus() != JobStatusFailed {
		t.Fatalf("err = %v status %s, want failed", err, job.GetStatus())
	}
	if !strings.Contains(job.GetError().Error(), "sprite sheet generation failed") {
		t.Fatalf("error = %v", job.GetError())
	}
}

type fakeAnimatedGenerator struct {
	plan      *AnimatedThumbnailsPlan
	prepErr   error
	committed *workerproto.AnimatedThumbnailsResult
	dir       string
	files     []string
	commit    AnimatedThumbnailsCommit
}

func (g *fakeAnimatedGenerator) PrepareAnimatedThumbnails(sceneID uint, forceTarget string) (*AnimatedThumbnailsPlan, error) {
	return g.plan, g.prepErr
}

func (g *fakeAnimatedGenerator) CommitAnimatedThumbnails(sceneID uint, res *workerproto.AnimatedThumbnailsResult, artifactDir string) (int, bool, error) {
	g.committed, g.dir = res, artifactDir
	entries, _ := os.ReadDir(artifactDir)
	for _, e := range entries {
		g.files = append(g.files, e.Name())
	}
	return CommitAnimatedThumbnails(g.commit, sceneID, res, artifactDir)
}

func TestAnimatedThumbnailJob_ComputesAndCommits(t *testing.T) {
	clip := renderClip(t, 12, true)
	root := t.TempDir()
	clips, previews := &fakeClipRepo{}, &fakePreviewRepo{}
	gen := &fakeAnimatedGenerator{
		plan: &AnimatedThumbnailsPlan{
			Input: clip,
			Video: workerproto.VideoInfo{DurationSec: 12, AudioCodec: "aac"},
			Params: workerproto.AnimatedThumbnailsParams{
				SceneID: 4, MarkerWidth: 160, MarkerCRF: 32,
				Markers: []workerproto.MarkerClipSpec{{MarkerID: 11, TimestampSec: 1, DurationSec: 2, Output: "marker_11.mp4"}},
				Preview: &workerproto.ScenePreviewSpec{Output: "4_preview.mp4", Segments: 4, SegmentDuration: 1, Width: 160, CRF: 30, WithAudio: true},
			},
		},
		commit: AnimatedThumbnailsCommit{Markers: clips, Scenes: previews, MarkerDir: filepath.Join(root, "markers"), PreviewDir: filepath.Join(root, "previews")},
	}
	job := NewAnimatedThumbnailJobWithID("job-anim", 4, "", gen, zap.NewNop())
	job.SetStagingRoot(filepath.Join(root, "staging"))
	if err := job.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if job.GetStatus() != JobStatusCompleted {
		t.Fatalf("status = %s", job.GetStatus())
	}
	if gen.dir != LocalStagingDir(filepath.Join(root, "staging"), "job-anim") || len(gen.files) != 2 {
		t.Fatalf("commit saw dir %s files %v", gen.dir, gen.files)
	}
	if clips.paths[11] != "marker_11.mp4" || previews.previews[4] != "4_preview.mp4" {
		t.Fatalf("db = %v %v", clips.paths, previews.previews)
	}
	if _, err := os.Stat(filepath.Join(root, "markers", "marker_11.mp4")); err != nil {
		t.Fatalf("clip not committed: %v", err)
	}
	if _, err := os.Stat(gen.dir); !os.IsNotExist(err) {
		t.Fatal("the staging directory must be removed after the job")
	}
}

func TestAnimatedThumbnailJob_NothingToDo(t *testing.T) {
	gen := &fakeAnimatedGenerator{}
	job := NewAnimatedThumbnailJob(4, gen, zap.NewNop())
	if err := job.Execute(); err != nil || job.GetStatus() != JobStatusCompleted {
		t.Fatalf("err = %v status %s", err, job.GetStatus())
	}
	if gen.committed != nil || gen.dir != "" {
		t.Fatal("nothing to make: no commit")
	}

	failing := &fakeAnimatedGenerator{prepErr: fmt.Errorf("db down")}
	job = NewAnimatedThumbnailJob(4, failing, zap.NewNop())
	if err := job.Execute(); err == nil || job.GetStatus() != JobStatusFailed {
		t.Fatalf("a preparation error must fail the job: %v %s", err, job.GetStatus())
	}
}

func TestAnimatedThumbnailJob_CancelledBeforeStart(t *testing.T) {
	job := NewAnimatedThumbnailJob(4, &fakeAnimatedGenerator{}, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := job.ExecuteWithContext(ctx); err == nil || job.GetStatus() != JobStatusCancelled {
		t.Fatalf("err = %v status %s", err, job.GetStatus())
	}
}

func TestFingerprintJob_ComputesAndCommits(t *testing.T) {
	clip := renderClip(t, 10, false)
	repo := &fakeFingerprintRepo{}
	job := NewFingerprintJobWithID("job-fp", 6, clip, "", workerproto.FingerprintModeAudioOnly, zap.NewNop())
	job.SetSceneDuration(10)
	job.SetCommitter(NewFingerprintCommitter(repo, zap.NewNop()))
	if err := job.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	res := job.GetResult()
	if job.GetStatus() != JobStatusCompleted || res.FingerprintTypeLabel() != "visual" || len(res.VisualFingerprint) == 0 {
		t.Fatalf("status %s result %+v", job.GetStatus(), res)
	}
	if repo.calls != 1 || repo.fpType != "visual" || len(repo.visual) != 8*len(res.VisualFingerprint) {
		t.Fatalf("commit saved %d times type %q", repo.calls, repo.fpType)
	}
}

func TestFingerprintJob_AudioFailureFailsInAudioOnlyMode(t *testing.T) {
	if err := ffmpeg.CheckInstallation(); err != nil {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	job := NewFingerprintJob(6, filepath.Join(t.TempDir(), "missing.mp4"), "aac", workerproto.FingerprintModeAudioOnly, zap.NewNop())
	if err := job.Execute(); err == nil || job.GetStatus() != JobStatusFailed {
		t.Fatalf("err = %v status %s", err, job.GetStatus())
	}
}
