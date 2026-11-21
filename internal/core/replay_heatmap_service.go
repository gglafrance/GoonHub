package core

import (
	"errors"
	"fmt"
	"math"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	// ReplayHeatmapBucketCount is the number of equal parts a scene is split into.
	ReplayHeatmapBucketCount = 100
	// MaxReplayHeatmapRanges caps the watched ranges accepted in one report.
	MaxReplayHeatmapRanges = 500
)

// WatchedRange is a part of a scene that was played continuously, in seconds.
type WatchedRange struct {
	Start float64
	End   float64
}

// ReplayHeatmap is a user's "most replayed" histogram of a scene. Each bucket holds the
// watched time in milliseconds that fell into that part of the scene; Buckets is empty
// when nothing was recorded.
type ReplayHeatmap struct {
	BucketCount int     `json:"bucket_count"`
	Buckets     []int32 `json:"buckets"`
}

// RecordResult tells whether a report was stored and returns the updated histogram.
type RecordResult struct {
	Recorded bool `json:"recorded"`
	ReplayHeatmap
}

// AccumulateWatchedRanges converts watched ranges into per-bucket watched time
// (milliseconds) for a scene of the given duration (seconds) split into bucketCount equal
// buckets. Each range adds its overlap with every bucket it crosses, so splitting a range in
// several reports gives the same result and parts between two ranges (seeked over) are not
// counted. Ranges are clamped to the scene; empty, reversed and non-finite ranges are
// ignored. Returns nil when the duration is unknown (<= 0) or nothing was watched.
func AccumulateWatchedRanges(ranges []WatchedRange, duration float64, bucketCount int) []int32 {
	if bucketCount <= 0 || !(duration > 0) || math.IsInf(duration, 0) {
		return nil
	}
	bucketLen := duration / float64(bucketCount)
	watched := make([]float64, bucketCount)
	for _, r := range ranges {
		if math.IsNaN(r.Start) || math.IsNaN(r.End) {
			continue
		}
		start := math.Max(r.Start, 0)
		end := math.Min(r.End, duration)
		if !(end > start) {
			continue
		}
		first := min(int(start/bucketLen), bucketCount-1)
		last := min(int(end/bucketLen), bucketCount-1)
		for i := first; i <= last; i++ {
			lo := math.Max(start, float64(i)*bucketLen)
			hi := math.Min(end, float64(i+1)*bucketLen)
			if i == bucketCount-1 {
				hi = end
			}
			if hi > lo {
				watched[i] += hi - lo
			}
		}
	}
	increments := make([]int32, bucketCount)
	nonZero := false
	for i, seconds := range watched {
		ms := math.Round(seconds * 1000)
		if ms > math.MaxInt32 {
			ms = math.MaxInt32
		}
		increments[i] = int32(ms)
		if increments[i] > 0 {
			nonZero = true
		}
	}
	if !nonZero {
		return nil
	}
	return increments
}

// ReplayHeatmapService records which parts of a scene each user watches and returns the
// resulting "most replayed" histogram.
type ReplayHeatmapService struct {
	repo         data.ReplayHeatmapRepository
	sceneRepo    data.SceneRepository
	settingsRepo data.UserSettingsRepository
	logger       *zap.Logger
}

func NewReplayHeatmapService(repo data.ReplayHeatmapRepository, sceneRepo data.SceneRepository, settingsRepo data.UserSettingsRepository, logger *zap.Logger) *ReplayHeatmapService {
	return &ReplayHeatmapService{
		repo:         repo,
		sceneRepo:    sceneRepo,
		settingsRepo: settingsRepo,
		logger:       logger,
	}
}

// Record adds watched ranges to the user's histogram of the scene. The scene duration is
// taken from its metadata, or from the player (clientDuration) while it is unknown; with no
// duration at all, or when the user turned recording off, nothing is stored.
func (s *ReplayHeatmapService) Record(userID, sceneID uint, ranges []WatchedRange, clientDuration float64) (*RecordResult, error) {
	if len(ranges) > MaxReplayHeatmapRanges {
		return nil, apperrors.NewValidationErrorWithField("ranges",
			fmt.Sprintf("at most %d ranges per report", MaxReplayHeatmapRanges))
	}

	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(sceneID)
		}
		return nil, fmt.Errorf("failed to get scene: %w", err)
	}

	enabled, err := s.recordingEnabled(userID)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return &RecordResult{ReplayHeatmap: ReplayHeatmap{BucketCount: ReplayHeatmapBucketCount, Buckets: []int32{}}}, nil
	}

	duration := float64(scene.Duration)
	if duration <= 0 {
		duration = clientDuration
	}
	increments := AccumulateWatchedRanges(ranges, duration, ReplayHeatmapBucketCount)
	if increments == nil {
		heatmap, err := s.Get(userID, sceneID)
		if err != nil {
			return nil, err
		}
		return &RecordResult{ReplayHeatmap: *heatmap}, nil
	}

	buckets, err := s.repo.AddBuckets(userID, sceneID, increments)
	if err != nil {
		s.logger.Error("Failed to record replay heatmap",
			zap.Uint("user_id", userID),
			zap.Uint("scene_id", sceneID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to record replay heatmap: %w", err)
	}
	return &RecordResult{Recorded: true, ReplayHeatmap: newReplayHeatmap(buckets)}, nil
}

// Get returns the user's histogram of the scene (empty when nothing was recorded).
func (s *ReplayHeatmapService) Get(userID, sceneID uint) (*ReplayHeatmap, error) {
	buckets, err := s.repo.GetBuckets(userID, sceneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get replay heatmap: %w", err)
	}
	heatmap := newReplayHeatmap(buckets)
	return &heatmap, nil
}

func (s *ReplayHeatmapService) recordingEnabled(userID uint) (bool, error) {
	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, fmt.Errorf("failed to get user settings: %w", err)
	}
	return settings.RecordReplayHeatmap, nil
}

// newReplayHeatmap wraps stored buckets; a histogram without any watched time is empty.
func newReplayHeatmap(buckets []int32) ReplayHeatmap {
	heatmap := ReplayHeatmap{BucketCount: ReplayHeatmapBucketCount, Buckets: []int32{}}
	for _, v := range buckets {
		if v > 0 {
			heatmap.Buckets = buckets
			heatmap.BucketCount = len(buckets)
			break
		}
	}
	return heatmap
}
