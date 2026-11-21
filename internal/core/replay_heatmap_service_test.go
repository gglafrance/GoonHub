package core

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// buckets builds a histogram of n buckets with the given non-zero values.
func buckets(n int, values map[int]int32) []int32 {
	out := make([]int32, n)
	for i, v := range values {
		out[i] = v
	}
	return out
}

func TestAccumulateWatchedRanges(t *testing.T) {
	tests := []struct {
		name     string
		ranges   []WatchedRange
		duration float64
		want     []int32
	}{
		{"inside one bucket", []WatchedRange{{10.2, 10.7}}, 100, buckets(100, map[int]int32{10: 500})},
		{"crossing bucket bounds", []WatchedRange{{10.5, 12.25}}, 100,
			buckets(100, map[int]int32{10: 500, 11: 1000, 12: 250})},
		{"seeking skips the part in between", []WatchedRange{{0, 2}, {50, 51.5}}, 100,
			buckets(100, map[int]int32{0: 1000, 1: 1000, 50: 1000, 51: 500})},
		{"rewatching adds up", []WatchedRange{{20, 22}, {20, 21}}, 100,
			buckets(100, map[int]int32{20: 2000, 21: 1000})},
		{"backward seek then replay", []WatchedRange{{30, 33}, {31, 32}}, 100,
			buckets(100, map[int]int32{30: 1000, 31: 2000, 32: 1000})},
		{"clamped to the scene", []WatchedRange{{-5, 1}, {99.5, 150}}, 100,
			buckets(100, map[int]int32{0: 1000, 99: 500})},
		{"end of the scene in the last bucket", []WatchedRange{{98, 100}}, 100,
			buckets(100, map[int]int32{98: 1000, 99: 1000})},
		{"unknown duration", []WatchedRange{{0, 10}}, 0, nil},
		{"negative duration", []WatchedRange{{0, 10}}, -1, nil},
		{"NaN duration", []WatchedRange{{0, 10}}, math.NaN(), nil},
		{"infinite duration", []WatchedRange{{0, 10}}, math.Inf(1), nil},
		{"empty, reversed and invalid ranges", []WatchedRange{{5, 5}, {8, 3}, {math.NaN(), 4}, {120, 130}}, 100, nil},
		{"no ranges", nil, 100, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AccumulateWatchedRanges(tt.ranges, tt.duration, 100)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v\nwant %v", nonZero(got), nonZero(tt.want))
			}
		})
	}
}

func TestAccumulateWatchedRanges_CoarseBuckets(t *testing.T) {
	// 10 min scene, 10 buckets of 60 s; 0-30 s then 45 s-2 min after a seek
	got := AccumulateWatchedRanges([]WatchedRange{{0, 30}, {45, 120}}, 600, 10)
	want := buckets(10, map[int]int32{0: 45000, 1: 60000})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAccumulateWatchedRanges_SplitReportsMatchOneRange(t *testing.T) {
	// A continuous watch reported in several chunks (periodic flushes) counts once
	whole := AccumulateWatchedRanges([]WatchedRange{{3.3, 47.9}}, 61.7, 100)
	split := AccumulateWatchedRanges([]WatchedRange{{3.3, 10.01}, {10.01, 29.5}, {29.5, 47.9}}, 61.7, 100)
	for i := range whole {
		if d := whole[i] - split[i]; d < -1 || d > 1 {
			t.Fatalf("bucket %d: whole %d, split %d", i, whole[i], split[i])
		}
	}
	var total int64
	for _, v := range whole {
		total += int64(v)
	}
	if total < 44599 || total > 44601 {
		t.Fatalf("total watched %d ms, want 44600", total)
	}
}

func TestAccumulateWatchedRanges_Saturates(t *testing.T) {
	got := AccumulateWatchedRanges([]WatchedRange{{0, 1e10}}, 1e10, 100)
	if got[0] != math.MaxInt32 || got[99] != math.MaxInt32 {
		t.Fatalf("expected saturated buckets, got %d and %d", got[0], got[99])
	}
}

func nonZero(b []int32) map[int]int32 {
	out := map[int]int32{}
	for i, v := range b {
		if v != 0 {
			out[i] = v
		}
	}
	return out
}

func newTestReplayHeatmapService(t *testing.T) (*ReplayHeatmapService, *mocks.MockReplayHeatmapRepository, *mocks.MockSceneRepository, *mocks.MockUserSettingsRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockReplayHeatmapRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	settingsRepo := mocks.NewMockUserSettingsRepository(ctrl)
	return NewReplayHeatmapService(repo, sceneRepo, settingsRepo, zap.NewNop()), repo, sceneRepo, settingsRepo
}

func TestReplayHeatmapRecord_UsesSceneDuration(t *testing.T) {
	svc, repo, sceneRepo, settingsRepo := newTestReplayHeatmapService(t)
	sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, Duration: 200}, nil)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{RecordReplayHeatmap: true}, nil)
	// 200 s scene: 2 s buckets; the player duration is ignored
	want := buckets(100, map[int]int32{5: 2000, 6: 1000})
	stored := buckets(100, map[int]int32{5: 4000, 6: 1000})
	repo.EXPECT().AddBuckets(uint(1), uint(7), want).Return(stored, nil)

	res, err := svc.Record(1, 7, []WatchedRange{{10, 13}}, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Recorded || res.BucketCount != 100 || !reflect.DeepEqual(res.Buckets, stored) {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestReplayHeatmapRecord_UnknownSceneDurationUsesPlayerDuration(t *testing.T) {
	svc, repo, sceneRepo, settingsRepo := newTestReplayHeatmapService(t)
	sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7}, nil)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, gorm.ErrRecordNotFound) // defaults: on
	want := buckets(100, map[int]int32{0: 1000})
	repo.EXPECT().AddBuckets(uint(1), uint(7), want).Return(want, nil)

	res, err := svc.Record(1, 7, []WatchedRange{{0, 1}}, 100)
	if err != nil || !res.Recorded {
		t.Fatalf("expected recorded, got %+v, %v", res, err)
	}
}

func TestReplayHeatmapRecord_NoDurationStoresNothing(t *testing.T) {
	svc, repo, sceneRepo, settingsRepo := newTestReplayHeatmapService(t)
	sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7}, nil)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{RecordReplayHeatmap: true}, nil)
	repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(nil, nil)

	res, err := svc.Record(1, 7, []WatchedRange{{0, 10}}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Recorded || res.Buckets == nil || len(res.Buckets) != 0 {
		t.Fatalf("expected nothing recorded and empty buckets, got %+v", res)
	}
}

func TestReplayHeatmapRecord_RecordingOff(t *testing.T) {
	svc, _, sceneRepo, settingsRepo := newTestReplayHeatmapService(t)
	sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, Duration: 100}, nil)
	settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{RecordReplayHeatmap: false}, nil)

	res, err := svc.Record(1, 7, []WatchedRange{{0, 10}}, 100)
	if err != nil || res.Recorded {
		t.Fatalf("expected nothing recorded, got %+v, %v", res, err)
	}
}

func TestReplayHeatmapRecord_Errors(t *testing.T) {
	t.Run("scene not found", func(t *testing.T) {
		svc, _, sceneRepo, _ := newTestReplayHeatmapService(t)
		sceneRepo.EXPECT().GetByID(uint(7)).Return(nil, gorm.ErrRecordNotFound)
		_, err := svc.Record(1, 7, []WatchedRange{{0, 1}}, 10)
		if !apperrors.IsNotFound(err) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
	t.Run("too many ranges", func(t *testing.T) {
		svc, _, _, _ := newTestReplayHeatmapService(t)
		_, err := svc.Record(1, 7, make([]WatchedRange, MaxReplayHeatmapRanges+1), 10)
		if !apperrors.IsValidation(err) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
	t.Run("settings error", func(t *testing.T) {
		svc, _, sceneRepo, settingsRepo := newTestReplayHeatmapService(t)
		sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, Duration: 100}, nil)
		settingsRepo.EXPECT().GetByUserID(uint(1)).Return(nil, fmt.Errorf("db down"))
		if _, err := svc.Record(1, 7, []WatchedRange{{0, 1}}, 10); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("repository error", func(t *testing.T) {
		svc, repo, sceneRepo, settingsRepo := newTestReplayHeatmapService(t)
		sceneRepo.EXPECT().GetByID(uint(7)).Return(&data.Scene{ID: 7, Duration: 100}, nil)
		settingsRepo.EXPECT().GetByUserID(uint(1)).Return(&data.UserSettings{RecordReplayHeatmap: true}, nil)
		repo.EXPECT().AddBuckets(uint(1), uint(7), gomock.Any()).Return(nil, fmt.Errorf("db down"))
		if _, err := svc.Record(1, 7, []WatchedRange{{0, 1}}, 10); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestReplayHeatmapGet(t *testing.T) {
	svc, repo, _, _ := newTestReplayHeatmapService(t)

	repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(nil, nil)
	res, err := svc.Get(1, 7)
	if err != nil || res.BucketCount != ReplayHeatmapBucketCount || res.Buckets == nil || len(res.Buckets) != 0 {
		t.Fatalf("expected empty heatmap, got %+v, %v", res, err)
	}

	// A histogram without any watched time is empty too
	repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(make([]int32, 100), nil)
	if res, _ = svc.Get(1, 7); len(res.Buckets) != 0 {
		t.Fatalf("expected empty heatmap, got %+v", res)
	}

	stored := buckets(100, map[int]int32{3: 1500})
	repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(stored, nil)
	if res, _ = svc.Get(1, 7); !reflect.DeepEqual(res.Buckets, stored) {
		t.Fatalf("unexpected heatmap %+v", res)
	}

	repo.EXPECT().GetBuckets(uint(1), uint(7)).Return(nil, fmt.Errorf("db down"))
	if _, err := svc.Get(1, 7); err == nil {
		t.Fatal("expected error")
	}
}
