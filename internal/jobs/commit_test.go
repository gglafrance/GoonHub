package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goonhub/internal/mocks"
	"goonhub/pkg/workerproto"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestStagingRoot(t *testing.T) {
	if got := StagingRoot("/custom/staging", "/data/metadata/sprites"); got != "/custom/staging" {
		t.Fatalf("configured staging root ignored: %s", got)
	}
	want := filepath.Join(filepath.Dir(filepath.Clean("./data/metadata/sprites")), "staging")
	if got := StagingRoot("", "./data/metadata/sprites"); got != want {
		t.Fatalf("StagingRoot = %s, want %s", got, want)
	}
	if got := LocalStagingDir("/s", "job-1"); got != filepath.Join("/s", "local", "job-1") {
		t.Fatalf("LocalStagingDir = %s", got)
	}
}

func TestMoveArtifact(t *testing.T) {
	stage, dest := t.TempDir(), filepath.Join(t.TempDir(), "out")

	writeFile(t, filepath.Join(stage, "a.webp"), "new")
	writeFile(t, filepath.Join(dest, "a.webp"), "old")
	if err := MoveArtifact(stage, dest, "a.webp"); err != nil {
		t.Fatalf("MoveArtifact: %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "a.webp")); got != "new" {
		t.Fatalf("destination = %q, want the new file", got)
	}
	if _, err := os.Stat(filepath.Join(stage, "a.webp")); !os.IsNotExist(err) {
		t.Fatal("staged file must be gone after the move")
	}

	// A second commit of the same artifact is a no-op
	if err := MoveArtifact(stage, dest, "a.webp"); err != nil {
		t.Fatalf("re-commit must be idempotent: %v", err)
	}
	if err := MoveArtifact(stage, dest, "missing.webp"); err == nil {
		t.Fatal("an artifact in neither place must fail")
	}
	for _, bad := range []string{"../escape.webp", "sub/a.webp", `..\a.webp`, ".hidden", ""} {
		if err := MoveArtifact(stage, dest, bad); !errors.Is(err, workerproto.ErrInvalidArtifactName) {
			t.Fatalf("MoveArtifact(%q) err = %v, want ErrInvalidArtifactName", bad, err)
		}
	}
}

func TestCopyThenRename(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.bin"), filepath.Join(dir, "dst.bin")
	writeFile(t, src, "payload")
	writeFile(t, dst, "previous")
	if err := copyThenRename(src, dst); err != nil {
		t.Fatalf("copyThenRename: %v", err)
	}
	if got := readFile(t, dst); got != "payload" {
		t.Fatalf("dst = %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestCommitSprites(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSceneRepository(ctrl)
	stage := t.TempDir()
	spriteDir, vttDir := filepath.Join(t.TempDir(), "sprites"), filepath.Join(t.TempDir(), "vtt")
	writeFile(t, filepath.Join(stage, "7_sheet_001.webp"), "s1")
	writeFile(t, filepath.Join(stage, "7_sheet_002.webp"), "s2")
	writeFile(t, filepath.Join(stage, "7_thumbnails.vtt"), "WEBVTT")
	res := &workerproto.SpritesResult{
		SpriteSheets: []string{"7_sheet_001.webp", "7_sheet_002.webp"}, VTT: "7_thumbnails.vtt", SheetCount: 2, FrameInterval: 5,
	}

	wantSheet, wantVtt := filepath.Join(spriteDir, "7_sheet_001.webp"), filepath.Join(vttDir, "7_thumbnails.vtt")
	repo.EXPECT().UpdateSprites(uint(7), wantSheet, wantVtt, 2).Return(nil).Times(2)

	got, err := CommitSprites(repo, spriteDir, vttDir, 7, res, stage)
	if err != nil {
		t.Fatalf("CommitSprites: %v", err)
	}
	if got.SpriteSheetPath != wantSheet || got.VttPath != wantVtt || got.SpriteSheetCount != 2 {
		t.Fatalf("result = %+v", got)
	}
	if readFile(t, filepath.Join(spriteDir, "7_sheet_002.webp")) != "s2" || readFile(t, wantVtt) != "WEBVTT" {
		t.Fatal("files not moved into place")
	}

	// Idempotent: the same commit again (e.g. a retried complete) succeeds
	if _, err := CommitSprites(repo, spriteDir, vttDir, 7, res, stage); err != nil {
		t.Fatalf("second commit: %v", err)
	}
}

func TestCommitSprites_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockSceneRepository(ctrl)
	stage := t.TempDir()
	spriteDir, vttDir := t.TempDir(), t.TempDir()

	// Names of another scene are refused before anything moves (no DB call expected)
	other := &workerproto.SpritesResult{SpriteSheets: []string{"8_sheet_001.webp"}, VTT: "7_thumbnails.vtt", SheetCount: 1}
	if _, err := CommitSprites(repo, spriteDir, vttDir, 7, other, stage); err == nil {
		t.Fatal("a sheet of another scene must be refused")
	}
	if _, err := CommitSprites(repo, spriteDir, vttDir, 7, nil, stage); err == nil {
		t.Fatal("a nil result must be refused")
	}

	// Missing artifact
	missing := &workerproto.SpritesResult{SpriteSheets: []string{"7_sheet_001.webp"}, VTT: "7_thumbnails.vtt", SheetCount: 1}
	if _, err := CommitSprites(repo, spriteDir, vttDir, 7, missing, stage); err == nil {
		t.Fatal("a missing sheet must fail")
	}

	// DB failure
	writeFile(t, filepath.Join(stage, "7_sheet_001.webp"), "s")
	writeFile(t, filepath.Join(stage, "7_thumbnails.vtt"), "v")
	repo.EXPECT().UpdateSprites(uint(7), gomock.Any(), gomock.Any(), 1).Return(errors.New("db down"))
	if _, err := CommitSprites(repo, spriteDir, vttDir, 7, missing, stage); err == nil || !strings.Contains(err.Error(), "failed to update sprites") {
		t.Fatalf("err = %v, want the DB error", err)
	}
}

type fakeClipRepo struct {
	mu      sync.Mutex
	paths   map[uint]string
	failFor uint
}

func (f *fakeClipRepo) UpdateAnimatedThumbnailPath(id uint, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id == f.failFor {
		return errors.New("db down")
	}
	if f.paths == nil {
		f.paths = map[uint]string{}
	}
	f.paths[id] = path
	return nil
}

type fakePreviewRepo struct{ previews map[uint]string }

func (f *fakePreviewRepo) UpdatePreviewVideoPath(id uint, p string) error {
	if f.previews == nil {
		f.previews = map[uint]string{}
	}
	f.previews[id] = p
	return nil
}

func TestCommitAnimatedThumbnails(t *testing.T) {
	stage := t.TempDir()
	markerDir, previewDir := filepath.Join(t.TempDir(), "markers"), filepath.Join(t.TempDir(), "previews")
	writeFile(t, filepath.Join(stage, "marker_1.mp4"), "m1")
	writeFile(t, filepath.Join(stage, "marker_2.mp4"), "m2")
	writeFile(t, filepath.Join(stage, "5_preview.mp4"), "p")
	clips, previews := &fakeClipRepo{failFor: 2}, &fakePreviewRepo{}
	c := AnimatedThumbnailsCommit{Markers: clips, Scenes: previews, MarkerDir: markerDir, PreviewDir: previewDir}

	res := &workerproto.AnimatedThumbnailsResult{
		Markers: []workerproto.MarkerClipResult{
			{MarkerID: 1, File: "marker_1.mp4"},
			{MarkerID: 2, File: "marker_2.mp4"},
			{MarkerID: 3, File: "marker_3.mp4"}, // never made it to the staging dir
		},
		Preview: &workerproto.ScenePreviewResult{File: "5_preview.mp4"},
	}
	committed, preview, err := CommitAnimatedThumbnails(c, 5, res, stage)
	if committed != 1 || !preview {
		t.Fatalf("committed = %d preview = %v, want 1 and true", committed, preview)
	}
	if err == nil || !strings.Contains(err.Error(), "marker 2") || !strings.Contains(err.Error(), "marker 3") {
		t.Fatalf("err = %v, want the failures of markers 2 and 3", err)
	}
	if clips.paths[1] != "marker_1.mp4" || previews.previews[5] != "5_preview.mp4" {
		t.Fatalf("db = %v %v", clips.paths, previews.previews)
	}
	if readFile(t, filepath.Join(markerDir, "marker_1.mp4")) != "m1" || readFile(t, filepath.Join(previewDir, "5_preview.mp4")) != "p" {
		t.Fatal("files not moved into place")
	}

	if n, p, err := CommitAnimatedThumbnails(c, 5, nil, stage); n != 0 || p || err != nil {
		t.Fatalf("nil result must be a no-op, got %d %v %v", n, p, err)
	}
}

type fakeFingerprintRepo struct {
	mu            sync.Mutex
	fpType        string
	audio, visual []byte
	calls         int
	err           error
}

func (f *fakeFingerprintRepo) UpdateFingerprint(id uint, fpType string, audio, visual []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.fpType, f.audio, f.visual = fpType, audio, visual
	return f.err
}

type fakeMatcher struct {
	mu        sync.Mutex
	indexed   []string
	processed [][]MatchResult
	matches   map[string][]MatchResult
	active    atomic.Int32
	overlap   atomic.Bool
}

func (m *fakeMatcher) enter() {
	if m.active.Add(1) > 1 {
		m.overlap.Store(true)
	}
	time.Sleep(time.Millisecond)
}
func (m *fakeMatcher) leave() { m.active.Add(-1) }

func (m *fakeMatcher) IndexFingerprint(sceneID uint, fpType string, audio []int32, visual []uint64) error {
	m.enter()
	defer m.leave()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexed = append(m.indexed, fpType)
	return nil
}

func (m *fakeMatcher) FindMatches(sceneID uint, fpType string, audio []int32, visual []uint64) ([]MatchResult, error) {
	m.enter()
	defer m.leave()
	if fpType == "audio" && len(audio) == 0 || fpType == "visual" && len(visual) == 0 {
		return nil, errors.New("wrong fingerprint passed")
	}
	return m.matches[fpType], nil
}

func (m *fakeMatcher) ProcessMatches(sceneID uint, matches []MatchResult) error {
	m.enter()
	defer m.leave()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processed = append(m.processed, matches)
	return nil
}

func TestFingerprintCommitter_SavesAndMatches(t *testing.T) {
	repo := &fakeFingerprintRepo{}
	matcher := &fakeMatcher{matches: map[string][]MatchResult{
		"audio":  {{SceneID: 2, ConfidenceScore: 0.5, MatchType: "audio"}},
		"visual": {{SceneID: 2, ConfidenceScore: 0.9, MatchType: "visual"}, {SceneID: 3, ConfidenceScore: 0.7, MatchType: "visual"}},
	}}
	c := NewFingerprintCommitter(repo, zap.NewNop())
	c.SetMatcher(matcher)

	res := &workerproto.FingerprintResult{AudioFingerprint: []int32{-1, 2}, VisualFingerprint: []uint64{0xff}}
	if err := c.Commit(1, res); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if repo.fpType != "dual" || len(repo.audio) != 8 || len(repo.visual) != 8 {
		t.Fatalf("saved %q audio %d bytes visual %d bytes", repo.fpType, len(repo.audio), len(repo.visual))
	}
	if repo.audio[0] != 0xff || repo.audio[4] != 2 || repo.visual[0] != 0xff {
		t.Fatalf("fingerprints must be stored little-endian: %v %v", repo.audio, repo.visual)
	}
	if strings.Join(matcher.indexed, ",") != "audio,visual" {
		t.Fatalf("indexed = %v", matcher.indexed)
	}
	if len(matcher.processed) != 1 || len(matcher.processed[0]) != 2 {
		t.Fatalf("processed = %+v, want scenes 2 and 3 once each", matcher.processed)
	}
	for _, m := range matcher.processed[0] {
		if m.SceneID == 2 && m.ConfidenceScore != 0.9 {
			t.Fatalf("dedup must keep the best match of scene 2: %+v", m)
		}
	}
}

func TestFingerprintCommitter_VisualOnlyStoresNilAudio(t *testing.T) {
	repo := &fakeFingerprintRepo{}
	c := NewFingerprintCommitter(repo, zap.NewNop())
	if err := c.Commit(1, &workerproto.FingerprintResult{VisualFingerprint: []uint64{1, 2}}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if repo.fpType != "visual" || repo.audio != nil || len(repo.visual) != 16 {
		t.Fatalf("saved %q audio %v visual %d bytes", repo.fpType, repo.audio, len(repo.visual))
	}
}

func TestFingerprintCommitter_SaveErrorStillMatches(t *testing.T) {
	repo := &fakeFingerprintRepo{err: errors.New("db down")}
	matcher := &fakeMatcher{}
	c := NewFingerprintCommitter(repo, zap.NewNop())
	c.SetMatcher(matcher)
	err := c.Commit(1, &workerproto.FingerprintResult{AudioFingerprint: []int32{1}})
	if err == nil || !strings.Contains(err.Error(), "failed to save fingerprint") {
		t.Fatalf("err = %v, want the save error", err)
	}
	if len(matcher.indexed) != 1 {
		t.Fatal("matching must still run when the save fails (as before)")
	}
	if c.Commit(1, nil) == nil {
		t.Fatal("a nil result must be refused")
	}
}

func TestFingerprintCommitter_Serialized(t *testing.T) {
	matcher := &fakeMatcher{}
	c := NewFingerprintCommitter(&fakeFingerprintRepo{}, zap.NewNop())
	c.SetMatcher(matcher)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			if err := c.Commit(id, &workerproto.FingerprintResult{AudioFingerprint: []int32{1}, VisualFingerprint: []uint64{1}}); err != nil {
				t.Errorf("Commit: %v", err)
			}
		}(uint(i + 1))
	}
	wg.Wait()
	if matcher.overlap.Load() {
		t.Fatal("fingerprint commits must not run concurrently")
	}
}
