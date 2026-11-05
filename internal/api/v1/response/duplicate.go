package response

import "goonhub/internal/core"

// DuplicateGroupResponse represents a duplicate group in the API response
type DuplicateGroupResponse struct {
	ID          uint                           `json:"id"`
	Status      string                         `json:"status"`
	SceneCount  int                            `json:"scene_count"`
	BestSceneID *uint                          `json:"best_scene_id"`
	Members     []DuplicateGroupMemberResponse `json:"members"`
	CreatedAt   string                         `json:"created_at"`
	UpdatedAt   string                         `json:"updated_at"`
	ResolvedAt  *string                        `json:"resolved_at,omitempty"`
}

// DuplicateGroupMemberResponse represents a member of a duplicate group
type DuplicateGroupMemberResponse struct {
	SceneID         uint    `json:"scene_id"`
	Title           string  `json:"title"`
	Duration        int     `json:"duration"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	VideoCodec      string  `json:"video_codec"`
	AudioCodec      string  `json:"audio_codec"`
	BitRate         int64   `json:"bit_rate"`
	Size            int64   `json:"size"`
	ThumbnailPath   string  `json:"thumbnail_path"`
	IsBest          bool    `json:"is_best"`
	ConfidenceScore float64 `json:"confidence_score"`
	MatchType       string  `json:"match_type"`
	IsTrashed       bool    `json:"is_trashed"`
	TrashedAt       *string `json:"trashed_at,omitempty"`
}

// DuplicateStatsResponse represents duplicate group statistics
type DuplicateStatsResponse struct {
	Unresolved int64 `json:"unresolved"`
	Resolved   int64 `json:"resolved"`
	Dismissed  int64 `json:"dismissed"`
	Total      int64 `json:"total"`
}

// DuplicationConfigResponse represents the duplication detection configuration
type DuplicationConfigResponse struct {
	AudioDensityThreshold   float64 `json:"audio_density_threshold"`
	AudioMinHashes          int     `json:"audio_min_hashes"`
	AudioMaxHashOccurrences int     `json:"audio_max_hash_occurrences"`
	AudioMinSpan            int     `json:"audio_min_span"`
	VisualHammingMax        int     `json:"visual_hamming_max"`
	VisualMinFrames         int     `json:"visual_min_frames"`
	VisualMinSpan           int     `json:"visual_min_span"`
	DeltaTolerance          int     `json:"delta_tolerance"`
	FingerprintMode         string  `json:"fingerprint_mode"`
}

// ToDuplicateGroupResponse converts a core DuplicateGroupWithScenes to an API response
func ToDuplicateGroupResponse(g core.DuplicateGroupWithScenes) DuplicateGroupResponse {
	members := make([]DuplicateGroupMemberResponse, len(g.Members))
	for i, m := range g.Members {
		member := DuplicateGroupMemberResponse{
			SceneID:         m.SceneID,
			Title:           m.Title,
			Duration:        m.Duration,
			Width:           m.Width,
			Height:          m.Height,
			VideoCodec:      m.VideoCodec,
			AudioCodec:      m.AudioCodec,
			BitRate:         m.BitRate,
			Size:            m.Size,
			ThumbnailPath:   m.ThumbnailPath,
			IsBest:          m.IsBest,
			ConfidenceScore: m.ConfidenceScore,
			MatchType:       m.MatchType,
			IsTrashed:       m.IsTrashed,
		}
		if m.TrashedAt != nil {
			s := m.TrashedAt.Format("2006-01-02T15:04:05Z")
			member.TrashedAt = &s
		}
		members[i] = member
	}

	resp := DuplicateGroupResponse{
		ID:          g.ID,
		Status:      g.Status,
		SceneCount:  g.SceneCount,
		BestSceneID: g.BestSceneID,
		Members:     members,
		CreatedAt:   g.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   g.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if g.ResolvedAt != nil {
		s := g.ResolvedAt.Format("2006-01-02T15:04:05Z")
		resp.ResolvedAt = &s
	}

	return resp
}
