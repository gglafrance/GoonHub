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

// StudioResolution is the studio an exact name resolves to.
type StudioResolution struct {
	Studio *data.Studio
	// MatchedAlias is the alias that matched ("" when the name matched).
	MatchedAlias string
	// Candidates is the number of studios with this name or alias (more than 1 = ambiguous,
	// Studio is then the best one: a name match before an alias match, the oldest first).
	Candidates int
}

// StudioMergeResult describes the merge of a source studio into a target studio (done or to do).
type StudioMergeResult struct {
	Target *data.Studio
	Source *data.Studio
	data.StudioMergeStats
	// AliasesAdded are the names the target gains as aliases (the source's name and aliases).
	AliasesAdded []string
	// LogoFromSource is true when the target has no logo and takes the source's (same for
	// the favicon and the poster, each taken when missing).
	LogoFromSource bool
}

// Resolve returns the studio whose name or one of whose aliases is name (exact match,
// case-insensitive). Not found is an apperrors.NotFoundError.
func (s *StudioService) Resolve(name string) (*StudioResolution, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperrors.NewValidationErrorWithField("name", "name is required")
	}
	matches, err := s.studioRepo.FindByNameOrAlias([]string{name}, 0)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to resolve studio", err)
	}
	if len(matches) == 0 {
		return nil, apperrors.ErrStudioNotFoundByName(name)
	}
	best, alias := bestStudioMatch(matches, name)
	return &StudioResolution{Studio: best, MatchedAlias: alias, Candidates: len(matches)}, nil
}

// bestStudioMatch picks a name match before an alias match (matches are oldest first).
func bestStudioMatch(matches []data.Studio, name string) (*data.Studio, string) {
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

// checkNameFree returns an EntityConflictError when another studio (than selfID) already
// has name as its name or one of its aliases.
func (s *StudioService) checkNameFree(selfID uint, name string) error {
	matches, err := s.studioRepo.FindByNameOrAlias([]string{strings.TrimSpace(name)}, selfID)
	if err != nil {
		return apperrors.NewInternalError("failed to check studio name", err)
	}
	if len(matches) == 0 {
		return nil
	}
	existing, _ := bestStudioMatch(matches, name)
	return apperrors.NewEntityConflictError("studio", "name", name, existing.Name, existing)
}

// prepareAliases normalizes the aliases of studio selfID (0 = new studio) named name, and
// handles those already used by another studio as its name or alias: an EntityConflictError
// when strict, dropped otherwise.
func (s *StudioService) prepareAliases(selfID uint, name string, aliases []string, strict bool) ([]string, error) {
	normalized, err := NormalizeAliases(name, aliases)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return normalized, nil
	}
	others, err := s.studioRepo.FindByNameOrAlias(normalized, selfID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check studio aliases", err)
	}
	if len(others) == 0 {
		return normalized, nil
	}
	kept := make([]string, 0, len(normalized))
	for _, alias := range normalized {
		owner := studioUsing(others, alias)
		if owner == nil {
			kept = append(kept, alias)
			continue
		}
		if strict {
			return nil, apperrors.NewEntityConflictError("studio", "aliases", alias, owner.Name, owner)
		}
		s.logger.Info("Alias dropped: already used by another studio",
			zap.String("studio", name), zap.String("alias", alias), zap.Uint("used_by", owner.ID))
	}
	return kept, nil
}

func studioUsing(studios []data.Studio, value string) *data.Studio {
	for i := range studios {
		if _, ok := matchedName(studios[i].Name, studios[i].Aliases, value); ok {
			return &studios[i]
		}
	}
	return nil
}

// loadMergePair loads the target and source studios of a merge.
func (s *StudioService) loadMergePair(targetID, sourceID uint) (*data.Studio, *data.Studio, error) {
	if targetID == sourceID {
		return nil, nil, apperrors.NewValidationError("cannot merge a studio into itself")
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

// mergedStudioTarget returns a copy of target with the source merged in: aliases and, when the
// target has none, the source's logo, favicon and poster.
func mergedStudioTarget(target, source *data.Studio) (*data.Studio, []string, bool) {
	merged := *target
	aliases, added := MergeAliases(target.Name, target.Aliases, source.Name, source.Aliases)
	merged.Aliases = pq.StringArray(aliases)
	logoFromSource := target.Logo == "" && source.Logo != ""
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&merged.Logo, source.Logo)
	fill(&merged.Favicon, source.Favicon)
	fill(&merged.Poster, source.Poster)
	return &merged, added, logoFromSource
}

// MergePreview describes what merging sourceID into targetID would do, without doing it.
func (s *StudioService) MergePreview(targetID, sourceID uint) (*StudioMergeResult, error) {
	target, source, err := s.loadMergePair(targetID, sourceID)
	if err != nil {
		return nil, err
	}
	stats, err := s.studioRepo.GetMergePreview(targetID, sourceID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to preview studio merge", err)
	}
	merged, added, logoFromSource := mergedStudioTarget(target, source)
	return &StudioMergeResult{Target: merged, Source: source, StudioMergeStats: *stats, AliasesAdded: added, LogoFromSource: logoFromSource}, nil
}

// Merge merges studio sourceID into studio targetID: the source's scenes, child studios,
// ratings and likes move to the target, the source's name and aliases become aliases of the
// target, the target keeps its logo unless it has none, and the source is deleted.
func (s *StudioService) Merge(targetID, sourceID uint) (*StudioMergeResult, error) {
	target, source, err := s.loadMergePair(targetID, sourceID)
	if err != nil {
		return nil, err
	}
	merged, added, logoFromSource := mergedStudioTarget(target, source)
	stats, sceneIDs, err := s.studioRepo.Merge(merged, source)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrStudioNotFound(sourceID)
		}
		return nil, apperrors.NewInternalError("failed to merge studios", err)
	}
	reindexScenesInBatches(s.sceneRepo, s.indexer, s.logger, sceneIDs)
	s.logger.Info("Studio merged",
		zap.Uint("source_id", source.ID), zap.String("source", source.Name),
		zap.Uint("target_id", merged.ID), zap.String("target", merged.Name),
		zap.Int64("scenes", stats.Scenes), zap.Int64("ratings", stats.Ratings), zap.Int64("likes", stats.Likes))
	return &StudioMergeResult{Target: merged, Source: source, StudioMergeStats: *stats, AliasesAdded: added, LogoFromSource: logoFromSource}, nil
}
