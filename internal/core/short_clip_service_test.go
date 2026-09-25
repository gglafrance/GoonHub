package core

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"goonhub/pkg/ffmpeg"
)

// fakeClipEncoder writes a small file in place of ffmpeg, or fails.
type fakeClipEncoder struct {
	err       error
	calls     int
	gotStart  float64
	gotLength float64
	gotOut    string
}

func (f *fakeClipEncoder) ExtractClip(ctx context.Context, in, out string, start, duration float64, onProgress func(pct float64)) error {
	f.calls++
	f.gotStart = start
	f.gotLength = duration
	f.gotOut = out
	if err := os.WriteFile(out, []byte("partial video data"), 0644); err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(0.5)
	}
	if f.err != nil {
		return f.err
	}
	return ctx.Err()
}

type shortClipTestEnv struct {
	svc       *ShortClipService
	sceneRepo *mocks.MockSceneRepository
	tagRepo   *mocks.MockTagRepository
	actorRepo *mocks.MockActorRepository
	encoder   *fakeClipEncoder
	saveDir   string
	eventBus  *EventBus
}

func newShortClipTestEnv(t *testing.T) *shortClipTestEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	actorRepo := mocks.NewMockActorRepository(ctrl)
	appRepo := mocks.NewMockAppSettingsRepository(ctrl)

	storageRoot := t.TempDir()
	appRepo.EXPECT().Get().Return(&data.AppSettingsRecord{ShortsMaxDuration: 60}, nil).AnyTimes()
	settings := NewShortsSettingsService(appRepo, &fakeStoragePaths{
		paths: []data.StoragePath{{ID: 7, Name: "Main", Path: storageRoot, IsDefault: true}},
		def:   &data.StoragePath{ID: 7, Name: "Main", Path: storageRoot, IsDefault: true},
	}, "shorts", "", zap.NewNop())

	sceneService := &SceneService{Repo: sceneRepo, logger: zap.NewNop()}
	eventBus := NewEventBus(zap.NewNop())
	svc := NewShortClipService(sceneService, tagRepo, actorRepo, settings, eventBus, ffmpeg.ClipOptions{}, 1, zap.NewNop())
	encoder := &fakeClipEncoder{}
	svc.encoder = encoder
	t.Cleanup(func() { svc.Shutdown(5 * time.Second) })

	return &shortClipTestEnv{
		svc:       svc,
		sceneRepo: sceneRepo,
		tagRepo:   tagRepo,
		actorRepo: actorRepo,
		encoder:   encoder,
		saveDir:   filepath.Join(storageRoot, "shorts"),
		eventBus:  eventBus,
	}
}

func testSourceScene(dir string) *data.Scene {
	studioID := uint(4)
	return &data.Scene{
		ID:         42,
		Title:      "Great Scene: Part 1/2",
		StoredPath: filepath.Join(dir, "source.mp4"),
		Duration:   300,
		Origin:     data.SceneOriginWeb,
		Type:       data.SceneTypeProfessional,
		Studio:     "Studio X",
		StudioID:   &studioID,
		Tags:       pq.StringArray{"tag-a"},
		Actors:     pq.StringArray{"Actor A"},
	}
}

func TestValidateShortRange(t *testing.T) {
	tests := []struct {
		name     string
		start    float64
		end      float64
		duration int
		wantErr  bool
	}{
		{"valid", 10, 20, 300, false},
		{"exactly one second", 10, 11, 300, false},
		{"from the start", 0, 5, 300, false},
		{"to the end", 290, 300, 300, false},
		{"slightly past stored duration", 290, 300.6, 300, false},
		{"longer than the feed limit is allowed", 0, 250, 300, false},
		{"under one second", 10, 10.5, 300, true},
		{"end equals start", 10, 10, 300, true},
		{"end before start", 20, 10, 300, true},
		{"negative start", -1, 10, 300, true},
		{"past the end", 290, 302, 300, true},
		{"unprocessed source", 0, 10, 0, true},
		{"NaN", math.NaN(), 10, 300, true},
		{"infinite end", 0, math.Inf(1), 300, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShortRange(tt.start, tt.end, tt.duration)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !apperrors.IsValidation(err) {
				t.Fatalf("expected a validation error, got %T", err)
			}
		})
	}
}

func TestShortNames(t *testing.T) {
	if got := DefaultShortTitle("My Scene", 83, 105.9); got != "My Scene (01:23-01:45)" {
		t.Fatalf("DefaultShortTitle = %q", got)
	}
	if got := DefaultShortTitle("Long", 3723, 3730); got != "Long (1:02:03-1:02:10)" {
		t.Fatalf("DefaultShortTitle past an hour = %q", got)
	}

	tests := []struct {
		title string
		want  string
	}{
		{"My Scene", "My Scene - short 01m23s-01m45s"},
		{`a/b\c:d*e?f"g<h>i|j`, "a b c d e f g h i j - short 01m23s-01m45s"},
		{"  spaced   out  ", "spaced out - short 01m23s-01m45s"},
		{"dots...", "dots - short 01m23s-01m45s"},
		{"tab\tand\nnewline", "tabandnewline - short 01m23s-01m45s"},
		{"", "scene 42 - short 01m23s-01m45s"},
		{"///", "scene 42 - short 01m23s-01m45s"},
	}
	for _, tt := range tests {
		if got := ShortFileBase(tt.title, 42, 83, 105); got != tt.want {
			t.Errorf("ShortFileBase(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}

	long := strings.Repeat("x", 400)
	if got := ShortFileBase(long, 1, 0, 1); len(got) > maxShortFileBaseLength+30 {
		t.Fatalf("file name not truncated: %d chars", len(got))
	}
	if got := ShortFileBase("t", 1, 3723, 3730); got != "t - short 1h02m03s-1h02m10s" {
		t.Fatalf("ShortFileBase past an hour = %q", got)
	}
}

func TestReserveShortPath_CollisionSuffixes(t *testing.T) {
	dir := t.TempDir()
	base := "Scene - short 00m10s-00m20s"

	final, partial, err := reserveShortPath(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(final) != base+".mp4" || partial != final+".partial" {
		t.Fatalf("first reservation = %q, %q", final, partial)
	}

	// The first name is now held by its .partial; a finished file holds (2).
	if err := os.WriteFile(filepath.Join(dir, base+" (2).mp4"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	final3, _, err := reserveShortPath(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(final3) != base+" (3).mp4" {
		t.Fatalf("third reservation = %q", final3)
	}
}

func TestShortClip_RunCopiesMetadataAndLinksSource(t *testing.T) {
	env := newShortClipTestEnv(t)
	source := testSourceScene(t.TempDir())

	env.sceneRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *data.Scene) error {
		s.ID = 100
		return nil
	})
	env.actorRepo.EXPECT().GetSceneActors(uint(42)).Return([]data.Actor{{ID: 5}, {ID: 6}}, nil)
	env.actorRepo.EXPECT().SetSceneActors(uint(100), []uint{5, 6}).Return(nil)
	env.tagRepo.EXPECT().GetSceneTags(uint(42)).Return([]data.Tag{{ID: 9}}, nil)
	env.tagRepo.EXPECT().SetSceneTags(uint(100), []uint{9}).Return(nil)

	storagePathID := uint(7)
	task := &ShortClipTask{
		TaskID: "t1", SourceSceneID: 42, Start: 83, End: 105, Title: "Clip title",
		source: source, saveDir: env.saveDir, storagePathID: &storagePathID,
	}

	scene, err := env.svc.encode(context.Background(), task)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if env.encoder.gotStart != 83 || env.encoder.gotLength != 22 {
		t.Fatalf("encoder got start %v length %v", env.encoder.gotStart, env.encoder.gotLength)
	}
	if !strings.HasSuffix(env.encoder.gotOut, ".mp4.partial") {
		t.Fatalf("encoder wrote to %q, want a .partial file", env.encoder.gotOut)
	}

	wantPath := filepath.Join(env.saveDir, "Great Scene Part 1 2 - short 01m23s-01m45s.mp4")
	if scene.StoredPath != wantPath {
		t.Fatalf("StoredPath = %q, want %q", scene.StoredPath, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
	if _, err := os.Stat(wantPath + ".partial"); !os.IsNotExist(err) {
		t.Fatalf(".partial file left behind")
	}

	if scene.SourceSceneID == nil || *scene.SourceSceneID != 42 {
		t.Fatalf("SourceSceneID = %v", scene.SourceSceneID)
	}
	if *scene.SourceStart != 83 || *scene.SourceEnd != 105 {
		t.Fatalf("source range = %v-%v", *scene.SourceStart, *scene.SourceEnd)
	}
	if scene.StoragePathID == nil || *scene.StoragePathID != 7 {
		t.Fatalf("StoragePathID = %v", scene.StoragePathID)
	}
	if scene.Title != "Clip title" || scene.Origin != data.SceneOriginWeb || scene.Type != data.SceneTypeProfessional {
		t.Fatalf("title/origin/type = %q %q %q", scene.Title, scene.Origin, scene.Type)
	}
	if scene.Studio != "Studio X" || scene.StudioID == nil || *scene.StudioID != 4 {
		t.Fatalf("studio = %q %v", scene.Studio, scene.StudioID)
	}
	if len(scene.Tags) != 1 || scene.Tags[0] != "tag-a" || len(scene.Actors) != 1 || scene.Actors[0] != "Actor A" {
		t.Fatalf("denormalized tags/actors = %v %v", scene.Tags, scene.Actors)
	}
	if scene.ProcessingStatus != "pending" || scene.Size == 0 {
		t.Fatalf("status = %q size = %d", scene.ProcessingStatus, scene.Size)
	}
	if !isCreatedClip(scene) {
		t.Fatalf("isCreatedClip = false for a created short")
	}
}

func TestShortClip_FailureLeavesNothing(t *testing.T) {
	env := newShortClipTestEnv(t)
	env.encoder.err = errors.New("ffmpeg exploded")
	// No Create expectation: gomock fails the test if a row is created.

	task := &ShortClipTask{
		TaskID: "t2", SourceSceneID: 42, Start: 0, End: 10, Title: "x",
		source: testSourceScene(t.TempDir()), saveDir: env.saveDir,
	}
	if _, err := env.svc.encode(context.Background(), task); err == nil {
		t.Fatal("expected an error")
	}

	entries, _ := os.ReadDir(env.saveDir)
	if len(entries) != 0 {
		t.Fatalf("files left behind: %v", entries)
	}
}

func TestShortClip_CancelLeavesNothing(t *testing.T) {
	env := newShortClipTestEnv(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := &ShortClipTask{
		TaskID: "t3", SourceSceneID: 42, Start: 0, End: 10, Title: "x",
		source: testSourceScene(t.TempDir()), saveDir: env.saveDir,
	}
	if _, err := env.svc.encode(ctx, task); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	entries, _ := os.ReadDir(env.saveDir)
	if len(entries) != 0 {
		t.Fatalf("files left behind: %v", entries)
	}
}

func TestShortClip_RowCreateFailureRemovesFile(t *testing.T) {
	env := newShortClipTestEnv(t)
	env.sceneRepo.EXPECT().Create(gomock.Any()).Return(errors.New("db down"))

	task := &ShortClipTask{
		TaskID: "t4", SourceSceneID: 42, Start: 0, End: 10, Title: "x",
		source: testSourceScene(t.TempDir()), saveDir: env.saveDir,
	}
	if _, err := env.svc.encode(context.Background(), task); err == nil {
		t.Fatal("expected an error")
	}
	entries, _ := os.ReadDir(env.saveDir)
	if len(entries) != 0 {
		t.Fatalf("files left behind: %v", entries)
	}
}

func TestShortClip_CreateShortValidation(t *testing.T) {
	t.Run("source not found", func(t *testing.T) {
		env := newShortClipTestEnv(t)
		env.sceneRepo.EXPECT().GetByID(uint(1)).Return(nil, gorm.ErrRecordNotFound)
		if _, err := env.svc.CreateShort(1, 0, 10, ""); !apperrors.IsNotFound(err) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("save folder not writable", func(t *testing.T) {
		env := newShortClipTestEnv(t)
		// A file where the shorts folder should be makes MkdirAll fail.
		if err := os.WriteFile(env.saveDir, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		env.sceneRepo.EXPECT().GetByID(uint(42)).Return(testSourceScene(t.TempDir()), nil)
		if _, err := env.svc.CreateShort(42, 0, 10, ""); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
		if env.encoder.calls != 0 {
			t.Fatal("encoder ran with an unwritable folder")
		}
	})

	t.Run("bad range", func(t *testing.T) {
		env := newShortClipTestEnv(t)
		env.sceneRepo.EXPECT().GetByID(uint(42)).Return(testSourceScene(t.TempDir()), nil)
		if _, err := env.svc.CreateShort(42, 10, 10.2, ""); !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
		if env.encoder.calls != 0 {
			t.Fatal("encoder ran for an invalid range")
		}
	})
}

func TestShortClip_CreateShortEndToEnd(t *testing.T) {
	env := newShortClipTestEnv(t)
	source := testSourceScene(t.TempDir())

	subID, events := env.eventBus.Subscribe()
	defer env.eventBus.Unsubscribe(subID)

	env.sceneRepo.EXPECT().GetByID(uint(42)).Return(source, nil)
	env.sceneRepo.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *data.Scene) error {
		s.ID = 101
		return nil
	})
	env.actorRepo.EXPECT().GetSceneActors(uint(42)).Return(nil, nil)
	env.tagRepo.EXPECT().GetSceneTags(uint(42)).Return(nil, nil)

	task, err := env.svc.CreateShort(42, 83, 105, "")
	if err != nil {
		t.Fatalf("CreateShort: %v", err)
	}
	if task.TaskID == "" || task.Title != "Great Scene: Part 1/2 (01:23-01:45)" {
		t.Fatalf("task = %+v", task)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			switch ev.Type {
			case "short:completed":
				payload := ev.Data.(map[string]any)
				if payload["task_id"] != task.TaskID || payload["scene_id"] != uint(101) {
					t.Fatalf("completed payload = %v", payload)
				}
				if len(env.svc.ListTasks(42)) != 0 {
					t.Fatal("task still listed after completion")
				}
				return
			case "short:failed":
				t.Fatalf("short failed: %v", ev.Data)
			}
		case <-deadline:
			t.Fatal("timed out waiting for short:completed")
		}
	}
}

func TestShortClip_CleanupStalePartials(t *testing.T) {
	env := newShortClipTestEnv(t)
	if err := os.MkdirAll(env.saveDir, 0755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(env.saveDir, "old - short 00m00s-00m10s.mp4.partial")
	keep := filepath.Join(env.saveDir, "done - short 00m00s-00m10s.mp4")
	for _, p := range []string{stale, keep} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	env.svc.CleanupStalePartials()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale .partial not removed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("finished short was removed")
	}
}
