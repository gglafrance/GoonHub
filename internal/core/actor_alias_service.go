package core

import (
	"errors"
	"strings"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// reindexBatchSize is the number of scenes re-indexed per search engine request after a merge.
const reindexBatchSize = 500

// ActorResolution is the actor an exact name resolves to.
type ActorResolution struct {
	Actor *data.Actor
	// MatchedAlias is the alias that matched ("" when the name matched).
	MatchedAlias string
	// Candidates is the number of actors with this name or alias (more than 1 = ambiguous,
	// Actor is then the best one: a name match before an alias match, the oldest first).
	Candidates int
}

// ActorMergeResult describes the merge of a source actor into a target actor (done or to do).
type ActorMergeResult struct {
	Target *data.Actor
	Source *data.Actor
	data.ActorMergeStats
	// AliasesAdded are the names the target gains as aliases (the source's name and aliases).
	AliasesAdded []string
	// ImageFromSource is true when the target has no photo and takes the source's.
	ImageFromSource bool
}

// Resolve returns the actor whose name or one of whose aliases is name (exact match,
// case-insensitive). Not found is an apperrors.NotFoundError.
func (s *ActorService) Resolve(name string) (*ActorResolution, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperrors.NewValidationErrorWithField("name", "name is required")
	}
	matches, err := s.actorRepo.FindByNameOrAlias([]string{name}, 0)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to resolve actor", err)
	}
	if len(matches) == 0 {
		return nil, apperrors.ErrActorNotFoundByName(name)
	}
	best, alias := bestActorMatch(matches, name)
	return &ActorResolution{Actor: best, MatchedAlias: alias, Candidates: len(matches)}, nil
}

// bestActorMatch picks a name match before an alias match (matches are oldest first).
func bestActorMatch(matches []data.Actor, name string) (*data.Actor, string) {
	for i := range matches {
		if strings.EqualFold(matches[i].Name, strings.TrimSpace(name)) {
			return &matches[i], ""
		}
	}
	for i := range matches {
		if alias, ok := matchedName(matches[i].Name, matches[i].Aliases, name); ok {
			return &matches[i], alias
		}
	}
	// the database folds case slightly differently than Go: trust it
	return &matches[0], ""
}

// checkNameFree returns an EntityConflictError when another actor (than selfID) already
// has name as its name or one of its aliases.
func (s *ActorService) checkNameFree(selfID uint, name string) error {
	matches, err := s.actorRepo.FindByNameOrAlias([]string{strings.TrimSpace(name)}, selfID)
	if err != nil {
		return apperrors.NewInternalError("failed to check actor name", err)
	}
	if len(matches) == 0 {
		return nil
	}
	existing, _ := bestActorMatch(matches, name)
	return apperrors.NewEntityConflictError("actor", "name", name, existing.Name, existing)
}

// prepareAliases normalizes the aliases of actor selfID (0 = new actor) named name, and
// handles those already used by another actor as its name or alias: an EntityConflictError
// when strict, dropped otherwise.
func (s *ActorService) prepareAliases(selfID uint, name string, aliases []string, strict bool) ([]string, error) {
	normalized, err := NormalizeAliases(name, aliases)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return normalized, nil
	}
	others, err := s.actorRepo.FindByNameOrAlias(normalized, selfID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check actor aliases", err)
	}
	if len(others) == 0 {
		return normalized, nil
	}
	kept := make([]string, 0, len(normalized))
	for _, alias := range normalized {
		owner := actorUsing(others, alias)
		if owner == nil {
			kept = append(kept, alias)
			continue
		}
		if strict {
			return nil, apperrors.NewEntityConflictError("actor", "aliases", alias, owner.Name, owner)
		}
		s.logger.Info("Alias dropped: already used by another actor",
			zap.String("actor", name), zap.String("alias", alias), zap.Uint("used_by", owner.ID))
	}
	return kept, nil
}

func actorUsing(actors []data.Actor, value string) *data.Actor {
	for i := range actors {
		if _, ok := matchedName(actors[i].Name, actors[i].Aliases, value); ok {
			return &actors[i]
		}
	}
	return nil
}

// loadMergePair loads the target and source actors of a merge.
func (s *ActorService) loadMergePair(targetID, sourceID uint) (*data.Actor, *data.Actor, error) {
	if targetID == sourceID {
		return nil, nil, apperrors.NewValidationError("cannot merge an actor into itself")
	}
	target, err := s.GetByID(targetID)
	if err != nil {
		return nil, nil, err
	}
	source, err := s.GetByID(sourceID)
	if err != nil {
		return nil, nil, err
	}
	return target, source, nil
}

// mergedTarget returns a copy of target with the source merged in: aliases and, when the
// target has none, the source's photo.
func mergedTarget(target, source *data.Actor) (*data.Actor, []string, bool) {
	merged := *target
	aliases, added := MergeAliases(target.Name, target.Aliases, source.Name, source.Aliases)
	merged.Aliases = pq.StringArray(aliases)
	imageFromSource := target.ImageURL == "" && source.ImageURL != ""
	if imageFromSource {
		merged.ImageURL = source.ImageURL
	}
	return &merged, added, imageFromSource
}

// MergePreview describes what merging sourceID into targetID would do, without doing it.
func (s *ActorService) MergePreview(targetID, sourceID uint) (*ActorMergeResult, error) {
	target, source, err := s.loadMergePair(targetID, sourceID)
	if err != nil {
		return nil, err
	}
	stats, err := s.actorRepo.GetMergePreview(targetID, sourceID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to preview actor merge", err)
	}
	merged, added, imageFromSource := mergedTarget(target, source)
	return &ActorMergeResult{Target: merged, Source: source, ActorMergeStats: *stats, AliasesAdded: added, ImageFromSource: imageFromSource}, nil
}

// Merge merges actor sourceID into actor targetID: the source's scenes, ratings and likes
// move to the target, the source's name and aliases become aliases of the target, the target
// keeps its photo unless it has none, and the source is deleted.
func (s *ActorService) Merge(targetID, sourceID uint) (*ActorMergeResult, error) {
	target, source, err := s.loadMergePair(targetID, sourceID)
	if err != nil {
		return nil, err
	}
	merged, added, imageFromSource := mergedTarget(target, source)
	stats, sceneIDs, err := s.actorRepo.Merge(merged, source)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrActorNotFound(sourceID)
		}
		return nil, apperrors.NewInternalError("failed to merge actors", err)
	}
	s.reindexScenes(sceneIDs)
	s.logger.Info("Actor merged",
		zap.Uint("source_id", source.ID), zap.String("source", source.Name),
		zap.Uint("target_id", merged.ID), zap.String("target", merged.Name),
		zap.Int64("scenes", stats.Scenes), zap.Int64("ratings", stats.Ratings), zap.Int64("likes", stats.Likes))
	return &ActorMergeResult{Target: merged, Source: source, ActorMergeStats: *stats, AliasesAdded: added, ImageFromSource: imageFromSource}, nil
}

// reindexScenes re-indexes scenes in the search engine, in batches (best effort).
func (s *ActorService) reindexScenes(sceneIDs []uint) {
	reindexScenesInBatches(s.sceneRepo, s.indexer, s.logger, sceneIDs)
}

func reindexScenesInBatches(sceneRepo data.SceneRepository, indexer SceneIndexer, logger *zap.Logger, sceneIDs []uint) {
	if indexer == nil {
		return
	}
	for start := 0; start < len(sceneIDs); start += reindexBatchSize {
		end := min(start+reindexBatchSize, len(sceneIDs))
		scenes, err := sceneRepo.GetByIDs(sceneIDs[start:end])
		if err != nil {
			logger.Warn("Failed to load scenes to re-index after merge", zap.Error(err))
			continue
		}
		if err := indexer.BulkUpdateSceneIndex(scenes); err != nil {
			logger.Warn("Failed to re-index scenes after merge", zap.Error(err))
		}
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
