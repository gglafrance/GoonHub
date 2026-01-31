package core

import (
	"goonhub/internal/data"
	"sort"

	"go.uber.org/zap"
)

// RelatedVideosService provides logic for finding videos related to a given video.
type RelatedVideosService struct {
	videoRepo  data.VideoRepository
	tagRepo    data.TagRepository
	actorRepo  data.ActorRepository
	studioRepo data.StudioRepository
	logger     *zap.Logger
}

// relatedVideoCandidate holds a video with its match score for sorting.
type relatedVideoCandidate struct {
	Video data.Video
	Score int
}

const (
	scoreActor    = 50
	scoreTag      = 30
	scoreStudio   = 20
	scoreFallback = 10
)

// NewRelatedVideosService creates a new RelatedVideosService.
func NewRelatedVideosService(
	videoRepo data.VideoRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	logger *zap.Logger,
) *RelatedVideosService {
	return &RelatedVideosService{
		videoRepo:  videoRepo,
		tagRepo:    tagRepo,
		actorRepo:  actorRepo,
		studioRepo: studioRepo,
		logger:     logger,
	}
}

// GetRelatedVideos returns videos related to the given video ID.
// Uses multi-phase matching: actors, tags, studio, then fallback to recent videos.
func (s *RelatedVideosService) GetRelatedVideos(videoID uint, limit int) ([]data.Video, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	// Track seen video IDs to avoid duplicates
	seenIDs := make(map[uint]bool)
	seenIDs[videoID] = true // Exclude source video

	var candidates []relatedVideoCandidate

	// Phase 1: Actor matches (up to 8 candidates)
	actorCandidates, err := s.findByActors(videoID, seenIDs, 8)
	if err != nil {
		s.logger.Warn("Failed to find related videos by actors",
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
	} else {
		candidates = append(candidates, actorCandidates...)
	}

	// Phase 2: Tag matches (up to 6 candidates)
	if len(candidates) < limit {
		tagCandidates, err := s.findByTags(videoID, seenIDs, 6)
		if err != nil {
			s.logger.Warn("Failed to find related videos by tags",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
		} else {
			candidates = append(candidates, tagCandidates...)
		}
	}

	// Phase 3: Studio matches (up to 4 candidates)
	if len(candidates) < limit {
		studioCandidates, err := s.findByStudio(videoID, seenIDs, 4)
		if err != nil {
			s.logger.Warn("Failed to find related videos by studio",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
		} else {
			candidates = append(candidates, studioCandidates...)
		}
	}

	// Phase 4: Fallback to recent videos if needed
	if len(candidates) < limit {
		needed := limit - len(candidates)
		fallbackCandidates, err := s.findFallback(seenIDs, needed)
		if err != nil {
			s.logger.Warn("Failed to find fallback videos",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
		} else {
			candidates = append(candidates, fallbackCandidates...)
		}
	}

	// Sort by score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Limit and extract videos
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]data.Video, len(candidates))
	for i, c := range candidates {
		result[i] = c.Video
	}

	return result, nil
}

// findByActors finds videos that share any actor with the source video.
func (s *RelatedVideosService) findByActors(videoID uint, seenIDs map[uint]bool, limit int) ([]relatedVideoCandidate, error) {
	// Get actors for the source video
	actors, err := s.actorRepo.GetVideoActors(videoID)
	if err != nil {
		return nil, err
	}
	if len(actors) == 0 {
		return nil, nil
	}

	var candidates []relatedVideoCandidate
	videoScores := make(map[uint]int) // Track score per video for multiple actor matches

	// For each actor, find their videos
	for _, actor := range actors {
		if len(candidates) >= limit {
			break
		}

		videos, _, err := s.actorRepo.GetActorVideos(actor.ID, 1, limit*2)
		if err != nil {
			s.logger.Debug("Failed to get videos for actor",
				zap.Uint("actor_id", actor.ID),
				zap.Error(err),
			)
			continue
		}

		for _, v := range videos {
			if seenIDs[v.ID] {
				videoScores[v.ID] += scoreActor // Add score for additional matches
				continue
			}
			seenIDs[v.ID] = true
			videoScores[v.ID] = scoreActor
			candidates = append(candidates, relatedVideoCandidate{
				Video: v,
				Score: scoreActor,
			})
		}
	}

	// Update scores for videos with multiple actor matches
	for i := range candidates {
		if score, ok := videoScores[candidates[i].Video.ID]; ok {
			candidates[i].Score = score
		}
	}

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates, nil
}

// findByTags finds videos that share any tag with the source video.
func (s *RelatedVideosService) findByTags(videoID uint, seenIDs map[uint]bool, limit int) ([]relatedVideoCandidate, error) {
	// Get tags for the source video
	tags, err := s.tagRepo.GetVideoTags(videoID)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}

	var candidates []relatedVideoCandidate
	videoScores := make(map[uint]int)

	// For each tag, find videos with that tag
	for _, tag := range tags {
		if len(candidates) >= limit {
			break
		}

		// Get video IDs for this tag
		videoIDs, err := s.getVideoIDsForTag(tag.ID, limit*2)
		if err != nil {
			s.logger.Debug("Failed to get videos for tag",
				zap.Uint("tag_id", tag.ID),
				zap.Error(err),
			)
			continue
		}

		// Filter out seen IDs and build list of new IDs to fetch
		var newIDs []uint
		for _, id := range videoIDs {
			if seenIDs[id] {
				videoScores[id] += scoreTag
				continue
			}
			newIDs = append(newIDs, id)
		}

		if len(newIDs) == 0 {
			continue
		}

		// Fetch the videos
		videos, err := s.videoRepo.GetByIDs(newIDs)
		if err != nil {
			s.logger.Debug("Failed to fetch videos by IDs",
				zap.Error(err),
			)
			continue
		}

		for _, v := range videos {
			if seenIDs[v.ID] {
				continue
			}
			seenIDs[v.ID] = true
			videoScores[v.ID] = scoreTag
			candidates = append(candidates, relatedVideoCandidate{
				Video: v,
				Score: scoreTag,
			})
		}
	}

	// Update scores for videos with multiple tag matches
	for i := range candidates {
		if score, ok := videoScores[candidates[i].Video.ID]; ok {
			candidates[i].Score = score
		}
	}

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates, nil
}

// getVideoIDsForTag returns video IDs that have the given tag.
func (s *RelatedVideosService) getVideoIDsForTag(tagID uint, limit int) ([]uint, error) {
	return s.tagRepo.GetVideoIDsByTag(tagID, limit)
}

// findByStudio finds videos from the same studio.
func (s *RelatedVideosService) findByStudio(videoID uint, seenIDs map[uint]bool, limit int) ([]relatedVideoCandidate, error) {
	// Get the video to find its studio
	video, err := s.videoRepo.GetByID(videoID)
	if err != nil {
		return nil, err
	}

	// Check if video has a studio
	if video.StudioID == nil {
		return nil, nil
	}

	// Get videos from the same studio
	videos, _, err := s.studioRepo.GetStudioVideos(*video.StudioID, 1, limit*2)
	if err != nil {
		return nil, err
	}

	var candidates []relatedVideoCandidate
	for _, v := range videos {
		if seenIDs[v.ID] {
			continue
		}
		seenIDs[v.ID] = true
		candidates = append(candidates, relatedVideoCandidate{
			Video: v,
			Score: scoreStudio,
		})
		if len(candidates) >= limit {
			break
		}
	}

	return candidates, nil
}

// findFallback returns recent videos as a fallback when other methods don't find enough.
func (s *RelatedVideosService) findFallback(seenIDs map[uint]bool, limit int) ([]relatedVideoCandidate, error) {
	// Fetch more than needed to account for filtering
	videos, _, err := s.videoRepo.List(1, limit*3)
	if err != nil {
		return nil, err
	}

	var candidates []relatedVideoCandidate
	for _, v := range videos {
		if seenIDs[v.ID] {
			continue
		}
		seenIDs[v.ID] = true
		candidates = append(candidates, relatedVideoCandidate{
			Video: v,
			Score: scoreFallback,
		})
		if len(candidates) >= limit {
			break
		}
	}

	return candidates, nil
}
