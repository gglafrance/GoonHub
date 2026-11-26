package core

import (
	"context"
	"fmt"

	"goonhub/internal/data"
)

// stats: the owner's watch figures over a period (watch time, views, videos watched, O) and
// totals (likes, markers, ratings), with their top tags, actors and studios by views. No file
// information (sizes) is ever part of it.

const profileStatsTopLimit = 5

// ProfileStatsPayload is the payload of the stats block.
type ProfileStatsPayload struct {
	Period        string                 `json:"period"`
	WatchTime     int64                  `json:"watch_time"`
	Views         int64                  `json:"views"`
	ScenesWatched int64                  `json:"scenes_watched"`
	Jizz          int64                  `json:"jizz"`
	Liked         int64                  `json:"liked"`
	Markers       int64                  `json:"markers"`
	Rated         int64                  `json:"rated"`
	AvgRating     *float64               `json:"avg_rating"`
	TopTags       []data.ProfileTopEntry `json:"top_tags"`
	TopActors     []data.ProfileTopEntry `json:"top_actors"`
	TopStudios    []data.ProfileTopEntry `json:"top_studios"`
}

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "stats",
		Label:            "Stats",
		Description:      "Watch time, views, O, likes and your top tags, actors and studios",
		PrivacyItem:      PrivacyStats,
		ViewerPermission: PermScenesView,
		Options:          []ProfileBlockOption{profilePeriodOption(profilePeriodAll)},
		DefaultVisible:   true,
		Order:            70,
		Fetch:            fetchStatsBlock,
	})
}

func fetchStatsBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	period := req.Config.String("period")
	since := profilePeriodSince(period, deps.now())
	st, err := deps.Repo.Stats(ctx, req.OwnerID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to read the stats: %w", err)
	}
	payload := ProfileStatsPayload{
		Period:        period,
		WatchTime:     st.WatchTime,
		Views:         st.Views,
		ScenesWatched: st.ScenesWatched,
		Jizz:          st.Jizz,
		Liked:         st.Liked,
		Markers:       st.Markers,
		Rated:         st.Rated,
		AvgRating:     st.AvgRating,
	}
	if payload.TopTags, err = deps.Repo.TopTags(ctx, req.OwnerID, since, profileStatsTopLimit); err != nil {
		return nil, fmt.Errorf("failed to read the top tags: %w", err)
	}
	if payload.TopActors, err = deps.Repo.TopActors(ctx, req.OwnerID, since, profileStatsTopLimit); err != nil {
		return nil, fmt.Errorf("failed to read the top actors: %w", err)
	}
	if payload.TopStudios, err = deps.Repo.TopStudios(ctx, req.OwnerID, since, profileStatsTopLimit); err != nil {
		return nil, fmt.Errorf("failed to read the top studios: %w", err)
	}
	empty := st.Views == 0 && st.Jizz == 0 && st.Liked == 0 && st.Markers == 0 && st.Rated == 0
	return &ProfileBlockData{Widget: payload, Empty: empty}, nil
}
