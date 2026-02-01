package core

import (
	"goonhub/internal/data"
	"sort"

	"go.uber.org/zap"
)

// RelatedScenesService provides logic for finding scenes related to a given scene.
type RelatedScenesService struct {
	sceneRepo  data.SceneRepository
	tagRepo    data.TagRepository
	actorRepo  data.ActorRepository
	studioRepo data.StudioRepository
	logger     *zap.Logger
}

// relatedSceneCandidate holds a scene with its match score for sorting.
type relatedSceneCandidate struct {
	Scene data.Scene
	Score int
}

const (
	scoreActor    = 50
	scoreTag      = 30
	scoreStudio   = 20
	scoreFallback = 10
)

// NewRelatedScenesService creates a new RelatedScenesService.
func NewRelatedScenesService(
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	logger *zap.Logger,
) *RelatedScenesService {
	return &RelatedScenesService{
		sceneRepo:  sceneRepo,
		tagRepo:    tagRepo,
		actorRepo:  actorRepo,
		studioRepo: studioRepo,
		logger:     logger,
	}
}

// GetRelatedScenes returns scenes related to the given scene ID.
// Uses multi-phase matching: actors, tags, studio, then fallback to recent scenes.
func (s *RelatedScenesService) GetRelatedScenes(sceneID uint, limit int) ([]data.Scene, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	// Track seen scene IDs to avoid duplicates
	seenIDs := make(map[uint]bool)
	seenIDs[sceneID] = true // Exclude source scene

	var candidates []relatedSceneCandidate

	// Phase 1: Actor matches (up to 8 candidates)
	actorCandidates, err := s.findByActors(sceneID, seenIDs, 8)
	if err != nil {
		s.logger.Warn("Failed to find related scenes by actors",
			zap.Uint("scene_id", sceneID),
			zap.Error(err),
		)
	} else {
		candidates = append(candidates, actorCandidates...)
	}

	// Phase 2: Tag matches (up to 6 candidates)
	if len(candidates) < limit {
		tagCandidates, err := s.findByTags(sceneID, seenIDs, 6)
		if err != nil {
			s.logger.Warn("Failed to find related scenes by tags",
				zap.Uint("scene_id", sceneID),
				zap.Error(err),
			)
		} else {
			candidates = append(candidates, tagCandidates...)
		}
	}

	// Phase 3: Studio matches (up to 4 candidates)
	if len(candidates) < limit {
		studioCandidates, err := s.findByStudio(sceneID, seenIDs, 4)
		if err != nil {
			s.logger.Warn("Failed to find related scenes by studio",
				zap.Uint("scene_id", sceneID),
				zap.Error(err),
			)
		} else {
			candidates = append(candidates, studioCandidates...)
		}
	}

	// Phase 4: Fallback to recent scenes if needed
	if len(candidates) < limit {
		needed := limit - len(candidates)
		fallbackCandidates, err := s.findFallback(seenIDs, needed)
		if err != nil {
			s.logger.Warn("Failed to find fallback scenes",
				zap.Uint("scene_id", sceneID),
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

	// Limit and extract scenes
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]data.Scene, len(candidates))
	for i, c := range candidates {
		result[i] = c.Scene
	}

	return result, nil
}

// findByActors finds scenes that share any actor with the source scene.
func (s *RelatedScenesService) findByActors(sceneID uint, seenIDs map[uint]bool, limit int) ([]relatedSceneCandidate, error) {
	// Get actors for the source scene
	actors, err := s.actorRepo.GetSceneActors(sceneID)
	if err != nil {
		return nil, err
	}
	if len(actors) == 0 {
		return nil, nil
	}

	var candidates []relatedSceneCandidate
	sceneScores := make(map[uint]int) // Track score per scene for multiple actor matches

	// For each actor, find their scenes
	for _, actor := range actors {
		if len(candidates) >= limit {
			break
		}

		scenes, _, err := s.actorRepo.GetActorScenes(actor.ID, 1, limit*2)
		if err != nil {
			s.logger.Debug("Failed to get scenes for actor",
				zap.Uint("actor_id", actor.ID),
				zap.Error(err),
			)
			continue
		}

		for _, v := range scenes {
			if seenIDs[v.ID] {
				sceneScores[v.ID] += scoreActor // Add score for additional matches
				continue
			}
			seenIDs[v.ID] = true
			sceneScores[v.ID] = scoreActor
			candidates = append(candidates, relatedSceneCandidate{
				Scene: v,
				Score: scoreActor,
			})
		}
	}

	// Update scores for scenes with multiple actor matches
	for i := range candidates {
		if score, ok := sceneScores[candidates[i].Scene.ID]; ok {
			candidates[i].Score = score
		}
	}

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates, nil
}

// findByTags finds scenes that share any tag with the source scene.
func (s *RelatedScenesService) findByTags(sceneID uint, seenIDs map[uint]bool, limit int) ([]relatedSceneCandidate, error) {
	// Get tags for the source scene
	tags, err := s.tagRepo.GetSceneTags(sceneID)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}

	var candidates []relatedSceneCandidate
	sceneScores := make(map[uint]int)

	// For each tag, find scenes with that tag
	for _, tag := range tags {
		if len(candidates) >= limit {
			break
		}

		// Get scene IDs for this tag
		sceneIDs, err := s.getSceneIDsForTag(tag.ID, limit*2)
		if err != nil {
			s.logger.Debug("Failed to get scenes for tag",
				zap.Uint("tag_id", tag.ID),
				zap.Error(err),
			)
			continue
		}

		// Filter out seen IDs and build list of new IDs to fetch
		var newIDs []uint
		for _, id := range sceneIDs {
			if seenIDs[id] {
				sceneScores[id] += scoreTag
				continue
			}
			newIDs = append(newIDs, id)
		}

		if len(newIDs) == 0 {
			continue
		}

		// Fetch the scenes
		scenes, err := s.sceneRepo.GetByIDs(newIDs)
		if err != nil {
			s.logger.Debug("Failed to fetch scenes by IDs",
				zap.Error(err),
			)
			continue
		}

		for _, v := range scenes {
			if seenIDs[v.ID] {
				continue
			}
			seenIDs[v.ID] = true
			sceneScores[v.ID] = scoreTag
			candidates = append(candidates, relatedSceneCandidate{
				Scene: v,
				Score: scoreTag,
			})
		}
	}

	// Update scores for scenes with multiple tag matches
	for i := range candidates {
		if score, ok := sceneScores[candidates[i].Scene.ID]; ok {
			candidates[i].Score = score
		}
	}

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates, nil
}

// getSceneIDsForTag returns scene IDs that have the given tag.
func (s *RelatedScenesService) getSceneIDsForTag(tagID uint, limit int) ([]uint, error) {
	return s.tagRepo.GetSceneIDsByTag(tagID, limit)
}

// findByStudio finds scenes from the same studio.
func (s *RelatedScenesService) findByStudio(sceneID uint, seenIDs map[uint]bool, limit int) ([]relatedSceneCandidate, error) {
	// Get the scene to find its studio
	scene, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		return nil, err
	}

	// Check if scene has a studio
	if scene.StudioID == nil {
		return nil, nil
	}

	// Get scenes from the same studio
	scenes, _, err := s.studioRepo.GetStudioScenes(*scene.StudioID, 1, limit*2)
	if err != nil {
		return nil, err
	}

	var candidates []relatedSceneCandidate
	for _, v := range scenes {
		if seenIDs[v.ID] {
			continue
		}
		seenIDs[v.ID] = true
		candidates = append(candidates, relatedSceneCandidate{
			Scene: v,
			Score: scoreStudio,
		})
		if len(candidates) >= limit {
			break
		}
	}

	return candidates, nil
}

// findFallback returns recent scenes as a fallback when other methods don't find enough.
func (s *RelatedScenesService) findFallback(seenIDs map[uint]bool, limit int) ([]relatedSceneCandidate, error) {
	// Fetch more than needed to account for filtering
	scenes, _, err := s.sceneRepo.List(1, limit*3)
	if err != nil {
		return nil, err
	}

	var candidates []relatedSceneCandidate
	for _, v := range scenes {
		if seenIDs[v.ID] {
			continue
		}
		seenIDs[v.ID] = true
		candidates = append(candidates, relatedSceneCandidate{
			Scene: v,
			Score: scoreFallback,
		})
		if len(candidates) >= limit {
			break
		}
	}

	return candidates, nil
}
