package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/lifecycle"
	"goonhub/pkg/ffmpeg"
)

const (
	// MinShortDuration is the shortest range that can be saved as a short, in seconds.
	MinShortDuration = 1.0
	// shortEndTolerance allows an end point slightly past the stored duration,
	// which is whole seconds while the player works in fractions.
	shortEndTolerance = 1.0
	// shortPartialExt is added to a short while ffmpeg writes it. It is not a
	// video extension, so a scan running at the same time ignores the file.
	shortPartialExt = ".partial"
	// shortProgressInterval limits how often progress events are published.
	shortProgressInterval = 500 * time.Millisecond
	// maxShortTitleLength matches scenes.title (VARCHAR(255)).
	maxShortTitleLength = 255
	// maxShortFileBaseLength keeps generated file names well under OS limits.
	maxShortFileBaseLength = 150
)

// Short task statuses.
const (
	ShortTaskQueued  = "queued"
	ShortTaskRunning = "running"
)

// clipEncoder cuts a range out of a video. It hides ffmpeg so tests can fake it.
type clipEncoder interface {
	ExtractClip(ctx context.Context, in, out string, start, duration float64, onProgress func(pct float64)) error
}

type ffmpegClipEncoder struct {
	opts ffmpeg.ClipOptions
}

func (e ffmpegClipEncoder) ExtractClip(ctx context.Context, in, out string, start, duration float64, onProgress func(pct float64)) error {
	return ffmpeg.ExtractClipWithContext(ctx, in, out, start, duration, e.opts, onProgress)
}

// ShortClipTask is a short being encoded.
type ShortClipTask struct {
	TaskID        string  `json:"task_id"`
	SourceSceneID uint    `json:"source_scene_id"`
	Start         float64 `json:"start"`
	End           float64 `json:"end"`
	Title         string  `json:"title"`
	Percent       int     `json:"percent"`
	Status        string  `json:"status"`

	source        *data.Scene
	saveDir       string
	storagePathID *uint
}

// ShortClipService cuts an A/B range of a scene into a new video file with
// ffmpeg and adds that file as a new scene linked to its source.
type ShortClipService struct {
	sceneService *SceneService
	sceneRepo    data.SceneRepository
	tagRepo      data.TagRepository
	actorRepo    data.ActorRepository
	settings     *ShortsSettingsService
	eventBus     *EventBus
	encoder      clipEncoder
	lifecycle    *lifecycle.Manager
	logger       *zap.Logger

	slots   chan struct{}
	running sync.WaitGroup

	mu    sync.Mutex
	tasks map[string]*ShortClipTask
}

// NewShortClipService creates a ShortClipService that encodes at most
// maxConcurrent shorts at once; the rest wait their turn.
func NewShortClipService(
	sceneService *SceneService,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	settings *ShortsSettingsService,
	eventBus *EventBus,
	opts ffmpeg.ClipOptions,
	maxConcurrent int,
	logger *zap.Logger,
) *ShortClipService {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	logger = logger.With(zap.String("component", "short_clip_service"))
	return &ShortClipService{
		sceneService: sceneService,
		sceneRepo:    sceneService.Repo,
		tagRepo:      tagRepo,
		actorRepo:    actorRepo,
		settings:     settings,
		eventBus:     eventBus,
		encoder:      ffmpegClipEncoder{opts: opts},
		lifecycle:    lifecycle.NewManager(logger),
		logger:       logger,
		slots:        make(chan struct{}, maxConcurrent),
		tasks:        make(map[string]*ShortClipTask),
	}
}

// ValidateShortRange checks a range against the source duration. There is no
// upper limit on length: a short longer than the feed limit is still saved.
func ValidateShortRange(start, end float64, sourceDuration int) error {
	if math.IsNaN(start) || math.IsNaN(end) || math.IsInf(start, 0) || math.IsInf(end, 0) {
		return apperrors.NewValidationError("start and end must be numbers")
	}
	if sourceDuration <= 0 {
		return apperrors.NewValidationError("the scene has not been processed yet, so its length is unknown")
	}
	if start < 0 {
		return apperrors.NewValidationErrorWithField("start", "set a start point: the start can't be before the beginning of the scene")
	}
	if end <= start {
		return apperrors.NewValidationErrorWithField("end", "set an end point after the start point")
	}
	if end > float64(sourceDuration)+shortEndTolerance {
		return apperrors.NewValidationErrorWithField("end", "the end point is past the end of the scene")
	}
	if end-start < MinShortDuration {
		return apperrors.NewValidationError("the short must be at least 1 second long")
	}
	return nil
}

// CreateShort validates the request and queues the encode. It returns as soon
// as the task is queued; progress arrives as short:* events.
func (s *ShortClipService) CreateShort(sourceID uint, start, end float64, title string) (*ShortClipTask, error) {
	source, err := s.sceneRepo.GetByID(sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(sourceID)
		}
		return nil, apperrors.NewInternalError("failed to load scene", err)
	}

	if err := ValidateShortRange(start, end, source.Duration); err != nil {
		return nil, err
	}

	dir, storagePathID, err := s.settings.ResolveSaveDir()
	if err != nil {
		return nil, err
	}
	// Refuse now rather than fail in the background: the folder can have
	// become unwritable since the settings were saved.
	if err := s.settings.ValidateSaveDir(dir); err != nil {
		return nil, err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		title = DefaultShortTitle(source.Title, start, end)
	}
	title = truncateRunes(title, maxShortTitleLength)

	task := &ShortClipTask{
		TaskID:        uuid.New().String(),
		SourceSceneID: source.ID,
		Start:         start,
		End:           end,
		Title:         title,
		Status:        ShortTaskQueued,
		source:        source,
		saveDir:       dir,
		storagePathID: storagePathID,
	}

	// Copy before the goroutine starts: it updates the task's status.
	snapshot := *task

	s.mu.Lock()
	s.tasks[task.TaskID] = task
	s.mu.Unlock()

	s.running.Add(1)
	s.lifecycle.GoWithContext(context.Background(), "short-clip-"+task.TaskID, func(ctx context.Context) {
		defer s.running.Done()
		s.run(ctx, task)
	})

	return &snapshot, nil
}

// ListTasks returns the shorts still being encoded for a source scene.
func (s *ShortClipService) ListTasks(sourceID uint) []ShortClipTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ShortClipTask, 0)
	for _, t := range s.tasks {
		if t.SourceSceneID == sourceID {
			out = append(out, *t)
		}
	}
	return out
}

// run encodes one short and turns it into a scene. On failure or cancel it
// leaves no file and no scene row behind.
func (s *ShortClipService) run(ctx context.Context, task *ShortClipTask) {
	defer func() {
		s.mu.Lock()
		delete(s.tasks, task.TaskID)
		s.mu.Unlock()
	}()

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		s.fail(task, ctx.Err())
		return
	}

	s.setStatus(task, ShortTaskRunning, 0)
	s.publishProgress(task, 0)

	scene, err := s.encode(ctx, task)
	if err != nil {
		s.fail(task, err)
		return
	}

	s.logger.Info("short created",
		zap.String("task_id", task.TaskID),
		zap.Uint("source_scene_id", task.SourceSceneID),
		zap.Uint("scene_id", scene.ID),
		zap.String("path", scene.StoredPath),
	)
	s.publish("short:completed", task.SourceSceneID, map[string]any{
		"task_id":         task.TaskID,
		"source_scene_id": task.SourceSceneID,
		"scene_id":        scene.ID,
	})
}

func (s *ShortClipService) encode(ctx context.Context, task *ShortClipTask) (*data.Scene, error) {
	if err := os.MkdirAll(task.saveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create shorts folder: %w", err)
	}

	finalPath, partialPath, err := reserveShortPath(task.saveDir, ShortFileBase(task.source.Title, task.source.ID, task.Start, task.End))
	if err != nil {
		return nil, err
	}

	var lastPublish time.Time
	lastPercent := -1
	onProgress := func(pct float64) {
		percent := int(pct * 100)
		if percent == lastPercent || time.Since(lastPublish) < shortProgressInterval {
			return
		}
		lastPercent = percent
		lastPublish = time.Now()
		s.setStatus(task, ShortTaskRunning, percent)
		s.publishProgress(task, percent)
	}

	if err := s.encoder.ExtractClip(ctx, task.source.StoredPath, partialPath, task.Start, task.End-task.Start, onProgress); err != nil {
		removeQuietly(partialPath)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		removeQuietly(partialPath)
		return nil, err
	}

	if err := os.Rename(partialPath, finalPath); err != nil {
		removeQuietly(partialPath)
		return nil, fmt.Errorf("failed to move short into place: %w", err)
	}

	scene := s.buildScene(task, finalPath)
	if err := s.sceneService.createSceneFromPath(scene, func(created *data.Scene) {
		s.copyMetadata(task.source.ID, created.ID)
	}); err != nil {
		removeQuietly(finalPath)
		return nil, fmt.Errorf("failed to create scene for short: %w", err)
	}

	return scene, nil
}

// buildScene makes the row for a finished short. Actors, tags and studio are
// copied onto the row's denormalized columns here; the join tables are filled
// by copyMetadata once the row exists.
func (s *ShortClipService) buildScene(task *ShortClipTask, path string) *data.Scene {
	source := task.source
	sourceID := source.ID
	start := task.Start
	end := task.End

	scene := &data.Scene{
		Title:            task.Title,
		OriginalFilename: filepath.Base(path),
		StoredPath:       path,
		StoragePathID:    task.storagePathID,
		ProcessingStatus: "pending",
		Origin:           source.Origin,
		Type:             source.Type,
		Studio:           source.Studio,
		StudioID:         source.StudioID,
		Tags:             append(pq.StringArray{}, source.Tags...),
		Actors:           append(pq.StringArray{}, source.Actors...),
		SourceSceneID:    &sourceID,
		SourceStart:      &start,
		SourceEnd:        &end,
	}
	if stat, err := os.Stat(path); err == nil {
		scene.Size = stat.Size()
	}
	return scene
}

// copyMetadata copies the source's actors and tags to the new short. A failure
// here is logged but does not undo the short.
func (s *ShortClipService) copyMetadata(sourceID, clipID uint) {
	if actors, err := s.actorRepo.GetSceneActors(sourceID); err != nil {
		s.logger.Warn("failed to read source actors for short", zap.Uint("source_scene_id", sourceID), zap.Error(err))
	} else if len(actors) > 0 {
		ids := make([]uint, len(actors))
		for i, a := range actors {
			ids[i] = a.ID
		}
		if err := s.actorRepo.SetSceneActors(clipID, ids); err != nil {
			s.logger.Warn("failed to copy actors to short", zap.Uint("scene_id", clipID), zap.Error(err))
		}
	}

	if tags, err := s.tagRepo.GetSceneTags(sourceID); err != nil {
		s.logger.Warn("failed to read source tags for short", zap.Uint("source_scene_id", sourceID), zap.Error(err))
	} else if len(tags) > 0 {
		ids := make([]uint, len(tags))
		for i, t := range tags {
			ids[i] = t.ID
		}
		if err := s.tagRepo.SetSceneTags(clipID, ids); err != nil {
			s.logger.Warn("failed to copy tags to short", zap.Uint("scene_id", clipID), zap.Error(err))
		}
	}
}

func (s *ShortClipService) fail(task *ShortClipTask, err error) {
	message := err.Error()
	if errors.Is(err, context.Canceled) {
		message = "cancelled: the server is shutting down"
	}
	s.logger.Warn("short failed",
		zap.String("task_id", task.TaskID),
		zap.Uint("source_scene_id", task.SourceSceneID),
		zap.Error(err),
	)
	s.publish("short:failed", task.SourceSceneID, map[string]any{
		"task_id":         task.TaskID,
		"source_scene_id": task.SourceSceneID,
		"error":           message,
	})
}

func (s *ShortClipService) setStatus(task *ShortClipTask, status string, percent int) {
	s.mu.Lock()
	task.Status = status
	task.Percent = percent
	s.mu.Unlock()
}

func (s *ShortClipService) publishProgress(task *ShortClipTask, percent int) {
	s.publish("short:progress", task.SourceSceneID, map[string]any{
		"task_id":         task.TaskID,
		"source_scene_id": task.SourceSceneID,
		"percent":         percent,
		"start":           task.Start,
		"end":             task.End,
		"title":           task.Title,
	})
}

func (s *ShortClipService) publish(eventType string, sourceID uint, payload map[string]any) {
	if s.eventBus == nil {
		return
	}
	s.eventBus.Publish(SceneEvent{Type: eventType, SceneID: sourceID, Data: payload})
}

// CleanupStalePartials removes half-written shorts left in the current shorts
// folder by a crash or a hard stop. Call it at startup, before any short runs.
func (s *ShortClipService) CleanupStalePartials() {
	dir, _, err := s.settings.ResolveSaveDir()
	if err != nil {
		return
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*"+shortPartialExt))
	if err != nil {
		return
	}
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			s.logger.Warn("failed to remove stale partial short", zap.String("path", m), zap.Error(err))
		} else {
			s.logger.Info("removed stale partial short", zap.String("path", m))
		}
	}
}

// Shutdown cancels running encodes and waits for them to clean up.
func (s *ShortClipService) Shutdown(timeout time.Duration) {
	_ = s.lifecycle.Shutdown(timeout)

	done := make(chan struct{})
	go func() {
		s.running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		s.logger.Warn("timed out waiting for shorts to stop")
	}
}

// reserveShortPath picks "<base>.mp4", then "<base> (2).mp4" and so on, and
// creates the matching ".partial" file so no other short can take the name.
func reserveShortPath(dir, base string) (finalPath, partialPath string, err error) {
	for n := 1; n < 1000; n++ {
		name := base + ".mp4"
		if n > 1 {
			name = fmt.Sprintf("%s (%d).mp4", base, n)
		}
		final := filepath.Join(dir, name)
		if _, statErr := os.Stat(final); statErr == nil {
			continue
		}
		partial := final + shortPartialExt
		f, createErr := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if createErr != nil {
			if os.IsExist(createErr) {
				continue
			}
			return "", "", fmt.Errorf("failed to create short file: %w", createErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			removeQuietly(partial)
			return "", "", fmt.Errorf("failed to create short file: %w", closeErr)
		}
		return final, partial, nil
	}
	return "", "", fmt.Errorf("no free file name for %q in %s", base, dir)
}

func removeQuietly(path string) {
	_ = os.Remove(path)
}

// DefaultShortTitle is "<source title> (01:23-01:45)".
func DefaultShortTitle(sourceTitle string, start, end float64) string {
	return fmt.Sprintf("%s (%s-%s)", strings.TrimSpace(sourceTitle), formatShortClock(start), formatShortClock(end))
}

// ShortFileBase is "<sanitized source title> - short 01m23s-01m45s", without
// the extension.
func ShortFileBase(sourceTitle string, sourceID uint, start, end float64) string {
	name := SanitizeFileName(sourceTitle)
	if name == "" {
		name = fmt.Sprintf("scene %d", sourceID)
	}
	name = truncateRunes(name, maxShortFileBaseLength)
	name = strings.TrimRight(name, ". ")
	return fmt.Sprintf("%s - short %s-%s", name, formatShortFileTime(start), formatShortFileTime(end))
}

// SanitizeFileName drops characters that are not allowed in file names on
// common file systems and collapses whitespace.
func SanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == utf8.RuneError, unicode.IsControl(r):
			continue
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	return strings.Trim(out, ". ")
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// formatShortClock is 01:23, or 1:02:03 from an hour on.
func formatShortClock(sec float64) string {
	h, m, s := splitSeconds(sec)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// formatShortFileTime is 01m23s, or 1h02m03s from an hour on.
func formatShortFileTime(sec float64) string {
	h, m, s := splitSeconds(sec)
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%02dm%02ds", m, s)
}

func splitSeconds(sec float64) (int, int, int) {
	if sec < 0 {
		sec = 0
	}
	total := int(sec)
	return total / 3600, (total % 3600) / 60, total % 60
}

// isCreatedClip reports whether a scene was cut from another scene. The range
// is checked too, because the source link is cleared if the source is deleted.
func isCreatedClip(scene *data.Scene) bool {
	return scene.SourceSceneID != nil || scene.SourceStart != nil
}
