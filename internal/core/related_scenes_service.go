package core

import (
	"fmt"
	"goonhub/internal/data"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// RelatedScenesService provides logic for finding scenes related to a given scene.
type RelatedScenesService struct {
	sceneRepo             data.SceneRepository
	tagRepo               data.TagRepository
	actorRepo             data.ActorRepository
	studioRepo            data.StudioRepository
	actorInteractionRepo  data.ActorInteractionRepository
	studioInteractionRepo data.StudioInteractionRepository
	watchHistoryRepo      data.WatchHistoryRepository
	logger                *zap.Logger
}

// relatedSceneCandidate holds a scene with its match score for sorting.
type relatedSceneCandidate struct {
	Scene data.Scene
	Score int
}

// Scoring constants
const (
	scorePerActor       = 40
	scoreLikedActorBonus = 25
	scorePerTag         = 8
	scoreStudioMatch    = 20
	scoreLikedStudioBonus = 15
	scoreTypeMatch      = 10
	scoreMaxPopularity  = 10
	scoreWatchedPenalty = -30
)

// Candidate pool caps per source
const (
	candidateCapActors = 200
	candidateCapTags   = 200
	candidateCapStudio = 50
)

// NewRelatedScenesService creates a new RelatedScenesService.
func NewRelatedScenesService(
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	actorInteractionRepo data.ActorInteractionRepository,
	studioInteractionRepo data.StudioInteractionRepository,
	watchHistoryRepo data.WatchHistoryRepository,
	logger *zap.Logger,
) *RelatedScenesService {
	return &RelatedScenesService{
		sceneRepo:             sceneRepo,
		tagRepo:               tagRepo,
		actorRepo:             actorRepo,
		studioRepo:            studioRepo,
		actorInteractionRepo:  actorInteractionRepo,
		studioInteractionRepo: studioInteractionRepo,
		watchHistoryRepo:      watchHistoryRepo,
		logger:                logger,
	}
}

// GetRelatedScenes returns scenes related to the given scene ID using a
// gather-then-score model. All signals (actors, tags, studio, type, popularity,
// user preferences) are accumulated for each candidate before ranking.
func (s *RelatedScenesService) GetRelatedScenes(sceneID uint, userID uint, limit int) ([]data.Scene, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	// Step 1: Fetch source scene data in parallel
	var sourceScene *data.Scene
	var sourceActors []data.Actor
	var sourceTags []data.Tag
	var sceneErr, actorErr, tagErr error

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		sourceScene, sceneErr = s.sceneRepo.GetByID(sceneID)
	}()
	go func() {
		defer wg.Done()
		sourceActors, actorErr = s.actorRepo.GetSceneActors(sceneID)
	}()
	go func() {
		defer wg.Done()
		sourceTags, tagErr = s.tagRepo.GetSceneTags(sceneID)
	}()
	wg.Wait()

	if sceneErr != nil {
		return nil, fmt.Errorf("failed to get source scene: %w", sceneErr)
	}
	if actorErr != nil {
		s.logger.Warn("failed to get source scene actors", zap.Uint("scene_id", sceneID), zap.Error(actorErr))
	}
	if tagErr != nil {
		s.logger.Warn("failed to get source scene tags", zap.Uint("scene_id", sceneID), zap.Error(tagErr))
	}

	// Step 2 & 3: Gather candidate IDs and user prefs in parallel
	candidateIDSet := make(map[uint]struct{})
	var mu sync.Mutex

	var likedActorSet map[uint]struct{}
	var likedStudioSet map[uint]struct{}
	var watchedSet map[uint]struct{}

	var wg2 sync.WaitGroup

	// Gather from actors
	for _, actor := range sourceActors {
		wg2.Add(1)
		go func(actorID uint) {
			defer wg2.Done()
			ids, err := s.actorRepo.GetActorSceneIDs(actorID)
			if err != nil {
				s.logger.Debug("failed to get scene IDs for actor", zap.Uint("actor_id", actorID), zap.Error(err))
				return
			}
			mu.Lock()
			for _, id := range ids {
				if len(candidateIDSet) >= candidateCapActors+candidateCapTags+candidateCapStudio {
					break
				}
				candidateIDSet[id] = struct{}{}
			}
			mu.Unlock()
		}(actor.ID)
	}

	// Gather from tags
	for _, tag := range sourceTags {
		wg2.Add(1)
		go func(tagID uint) {
			defer wg2.Done()
			ids, err := s.tagRepo.GetSceneIDsByTag(tagID, candidateCapTags)
			if err != nil {
				s.logger.Debug("failed to get scene IDs for tag", zap.Uint("tag_id", tagID), zap.Error(err))
				return
			}
			mu.Lock()
			for _, id := range ids {
				candidateIDSet[id] = struct{}{}
			}
			mu.Unlock()
		}(tag.ID)
	}

	// Gather from studio
	if sourceScene.StudioID != nil {
		wg2.Add(1)
		go func(studioID uint) {
			defer wg2.Done()
			ids, err := s.studioRepo.GetStudioSceneIDs(studioID, candidateCapStudio)
			if err != nil {
				s.logger.Debug("failed to get scene IDs for studio", zap.Uint("studio_id", studioID), zap.Error(err))
				return
			}
			mu.Lock()
			for _, id := range ids {
				candidateIDSet[id] = struct{}{}
			}
			mu.Unlock()
		}(*sourceScene.StudioID)
	}

	// Fetch user preferences (if logged in)
	if userID > 0 {
		wg2.Add(3)
		go func() {
			defer wg2.Done()
			ids, err := s.actorInteractionRepo.GetLikedActorIDs(userID)
			if err != nil {
				s.logger.Debug("failed to get liked actor IDs", zap.Uint("user_id", userID), zap.Error(err))
				return
			}
			set := make(map[uint]struct{}, len(ids))
			for _, id := range ids {
				set[id] = struct{}{}
			}
			mu.Lock()
			likedActorSet = set
			mu.Unlock()
		}()
		go func() {
			defer wg2.Done()
			ids, err := s.studioInteractionRepo.GetLikedStudioIDs(userID)
			if err != nil {
				s.logger.Debug("failed to get liked studio IDs", zap.Uint("user_id", userID), zap.Error(err))
				return
			}
			set := make(map[uint]struct{}, len(ids))
			for _, id := range ids {
				set[id] = struct{}{}
			}
			mu.Lock()
			likedStudioSet = set
			mu.Unlock()
		}()
		go func() {
			defer wg2.Done()
			ids, err := s.watchHistoryRepo.GetWatchedSceneIDs(userID, 500)
			if err != nil {
				s.logger.Debug("failed to get watched scene IDs", zap.Uint("user_id", userID), zap.Error(err))
				return
			}
			set := make(map[uint]struct{}, len(ids))
			for _, id := range ids {
				set[id] = struct{}{}
			}
			mu.Lock()
			watchedSet = set
			mu.Unlock()
		}()
	}

	wg2.Wait()

	// Remove source scene from candidates
	delete(candidateIDSet, sceneID)

	if len(candidateIDSet) == 0 {
		return s.fallbackPopular(sceneID, limit)
	}

	// Step 4: Build ID slice for batch fetch
	candidateIDs := make([]uint, 0, len(candidateIDSet))
	for id := range candidateIDSet {
		candidateIDs = append(candidateIDs, id)
	}

	// Step 5: Batch-fetch scene data, tags, and actors in parallel
	var scenes []data.Scene
	var tagsByScene map[uint][]data.Tag
	var actorsByScene map[uint][]data.Actor
	var scenesErr, tagsErr, actorsErr error

	var wg3 sync.WaitGroup
	wg3.Add(3)
	go func() {
		defer wg3.Done()
		scenes, scenesErr = s.sceneRepo.GetByIDs(candidateIDs)
	}()
	go func() {
		defer wg3.Done()
		tagsByScene, tagsErr = s.tagRepo.GetSceneTagsMultiple(candidateIDs)
	}()
	go func() {
		defer wg3.Done()
		actorsByScene, actorsErr = s.actorRepo.GetSceneActorsMultiple(candidateIDs)
	}()
	wg3.Wait()

	if scenesErr != nil {
		return nil, fmt.Errorf("failed to batch-fetch candidate scenes: %w", scenesErr)
	}
	if tagsErr != nil {
		s.logger.Warn("failed to batch-fetch candidate tags", zap.Error(tagsErr))
	}
	if actorsErr != nil {
		s.logger.Warn("failed to batch-fetch candidate actors", zap.Error(actorsErr))
	}

	if len(scenes) == 0 {
		return s.fallbackPopular(sceneID, limit)
	}

	// Build source data lookups
	sourceActorIDs := make(map[uint]struct{}, len(sourceActors))
	for _, a := range sourceActors {
		sourceActorIDs[a.ID] = struct{}{}
	}
	sourceTagIDs := make(map[uint]struct{}, len(sourceTags))
	for _, t := range sourceTags {
		sourceTagIDs[t.ID] = struct{}{}
	}

	// Find max view count for popularity normalization
	var maxViewCount int64
	for _, sc := range scenes {
		if sc.ViewCount > maxViewCount {
			maxViewCount = sc.ViewCount
		}
	}

	// Step 6: Score every candidate
	candidates := make([]relatedSceneCandidate, 0, len(scenes))
	for _, sc := range scenes {
		score := 0

		// Actor score
		if candidateActors, ok := actorsByScene[sc.ID]; ok {
			for _, ca := range candidateActors {
				if _, shared := sourceActorIDs[ca.ID]; shared {
					score += scorePerActor
					if likedActorSet != nil {
						if _, liked := likedActorSet[ca.ID]; liked {
							score += scoreLikedActorBonus
						}
					}
				}
			}
		}

		// Tag score
		if candidateTags, ok := tagsByScene[sc.ID]; ok {
			for _, ct := range candidateTags {
				if _, shared := sourceTagIDs[ct.ID]; shared {
					score += scorePerTag
				}
			}
		}

		// Studio score
		if sourceScene.StudioID != nil && sc.StudioID != nil && *sourceScene.StudioID == *sc.StudioID {
			score += scoreStudioMatch
			if likedStudioSet != nil {
				if _, liked := likedStudioSet[*sc.StudioID]; liked {
					score += scoreLikedStudioBonus
				}
			}
		}

		// Type score
		if sourceScene.Type != "" && sc.Type != "" && sourceScene.Type == sc.Type {
			score += scoreTypeMatch
		}

		// Popularity score (normalized 0-10)
		if maxViewCount > 0 {
			score += int(float64(sc.ViewCount) / float64(maxViewCount) * float64(scoreMaxPopularity))
		}

		// Watched penalty
		if watchedSet != nil {
			if _, watched := watchedSet[sc.ID]; watched {
				score += scoreWatchedPenalty
			}
		}

		if score < 0 {
			score = 0
		}

		candidates = append(candidates, relatedSceneCandidate{
			Scene: sc,
			Score: score,
		})
	}

	// Step 7: Sort by score desc, take top limit
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]data.Scene, len(candidates))
	for i, c := range candidates {
		result[i] = c.Scene
	}

	// Step 8: Fill with popular scenes if under limit
	if len(result) < limit {
		result = s.fillWithPopular(result, sceneID, limit)
	}

	return result, nil
}

// fallbackPopular returns popular scenes when no candidates are found.
func (s *RelatedScenesService) fallbackPopular(excludeID uint, limit int) ([]data.Scene, error) {
	popular, err := s.sceneRepo.ListPopular(limit + 1)
	if err != nil {
		s.logger.Warn("failed to get popular scenes for fallback", zap.Error(err))
		return []data.Scene{}, nil
	}

	result := make([]data.Scene, 0, limit)
	for _, sc := range popular {
		if sc.ID == excludeID {
			continue
		}
		result = append(result, sc)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

// fillWithPopular appends popular scenes to fill up to limit.
func (s *RelatedScenesService) fillWithPopular(existing []data.Scene, excludeID uint, limit int) []data.Scene {
	needed := limit - len(existing)
	if needed <= 0 {
		return existing
	}

	seenIDs := make(map[uint]struct{}, len(existing)+1)
	seenIDs[excludeID] = struct{}{}
	for _, sc := range existing {
		seenIDs[sc.ID] = struct{}{}
	}

	popular, err := s.sceneRepo.ListPopular(needed + len(seenIDs))
	if err != nil {
		s.logger.Warn("failed to get popular scenes for fill", zap.Error(err))
		return existing
	}

	for _, sc := range popular {
		if _, seen := seenIDs[sc.ID]; seen {
			continue
		}
		existing = append(existing, sc)
		seenIDs[sc.ID] = struct{}{}
		if len(existing) >= limit {
			break
		}
	}

	return existing
}
