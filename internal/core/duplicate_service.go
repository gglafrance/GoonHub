package core

import (
	"fmt"
	"goonhub/internal/data"
	"time"

	"go.uber.org/zap"
)

// DuplicateGroupWithScenes represents a duplicate group with full scene details for API responses
type DuplicateGroupWithScenes struct {
	ID          uint                         `json:"id"`
	Status      string                       `json:"status"`
	SceneCount  int                          `json:"scene_count"`
	BestSceneID *uint                        `json:"best_scene_id"`
	Members     []DuplicateGroupMemberDetail `json:"members"`
	CreatedAt   time.Time                    `json:"created_at"`
	UpdatedAt   time.Time                    `json:"updated_at"`
	ResolvedAt  *time.Time                   `json:"resolved_at,omitempty"`
}

// DuplicateGroupMemberDetail includes full scene data for a group member
type DuplicateGroupMemberDetail struct {
	SceneID         uint       `json:"scene_id"`
	Title           string     `json:"title"`
	Duration        int        `json:"duration"`
	Width           int        `json:"width"`
	Height          int        `json:"height"`
	VideoCodec      string     `json:"video_codec"`
	AudioCodec      string     `json:"audio_codec"`
	BitRate         int64      `json:"bit_rate"`
	Size            int64      `json:"size"`
	ThumbnailPath   string     `json:"thumbnail_path"`
	IsBest          bool       `json:"is_best"`
	ConfidenceScore float64    `json:"confidence_score"`
	MatchType       string     `json:"match_type"`
	IsTrashed       bool       `json:"is_trashed"`
	TrashedAt       *time.Time `json:"trashed_at,omitempty"`
}

// DuplicateStats holds stats by status
type DuplicateStats struct {
	Unresolved int64 `json:"unresolved"`
	Resolved   int64 `json:"resolved"`
	Dismissed  int64 `json:"dismissed"`
	Total      int64 `json:"total"`
}

// DuplicateService manages duplicate group operations
type DuplicateService struct {
	groupRepo       data.DuplicateGroupRepository
	sceneRepo       data.SceneRepository
	tagRepo         data.TagRepository
	actorRepo       data.ActorRepository
	markerRepo      data.MarkerRepository
	interactionRepo data.InteractionRepository
	eventBus        *EventBus
	logger          *zap.Logger
}

// NewDuplicateService creates a new DuplicateService
func NewDuplicateService(
	groupRepo data.DuplicateGroupRepository,
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	markerRepo data.MarkerRepository,
	interactionRepo data.InteractionRepository,
	eventBus *EventBus,
	logger *zap.Logger,
) *DuplicateService {
	return &DuplicateService{
		groupRepo:       groupRepo,
		sceneRepo:       sceneRepo,
		tagRepo:         tagRepo,
		actorRepo:       actorRepo,
		markerRepo:      markerRepo,
		interactionRepo: interactionRepo,
		eventBus:        eventBus,
		logger:          logger.With(zap.String("component", "duplicate_service")),
	}
}

// ListGroups returns paginated duplicate groups with member details
func (ds *DuplicateService) ListGroups(page, limit int, status, sortBy string) ([]DuplicateGroupWithScenes, int64, error) {
	groups, total, err := ds.groupRepo.ListWithMembers(page, limit, status)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list groups: %w", err)
	}

	result := make([]DuplicateGroupWithScenes, 0, len(groups))
	for _, g := range groups {
		enriched, err := ds.enrichGroup(&g)
		if err != nil {
			ds.logger.Error("Failed to enrich group", zap.Uint("group_id", g.ID), zap.Error(err))
			continue
		}
		result = append(result, *enriched)
	}

	return result, total, nil
}

// GetGroup returns a single group with full details
func (ds *DuplicateService) GetGroup(groupID uint) (*DuplicateGroupWithScenes, error) {
	group, err := ds.groupRepo.GetByIDWithMembers(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	return ds.enrichGroup(group)
}

// GetStats returns counts by status
func (ds *DuplicateService) GetStats() (*DuplicateStats, error) {
	counts, err := ds.groupRepo.CountByStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats := &DuplicateStats{
		Unresolved: counts["unresolved"],
		Resolved:   counts["resolved"],
		Dismissed:  counts["dismissed"],
	}
	stats.Total = stats.Unresolved + stats.Resolved + stats.Dismissed
	return stats, nil
}

// ScoreBestVariant determines the best variant in a group
func (ds *DuplicateService) ScoreBestVariant(groupID uint) (uint, error) {
	group, err := ds.groupRepo.GetByIDWithMembers(groupID)
	if err != nil {
		return 0, fmt.Errorf("failed to get group: %w", err)
	}

	var bestSceneID uint
	var bestScore int64

	for _, member := range group.Members {
		scene, err := ds.sceneRepo.GetByID(member.SceneID)
		if err != nil {
			continue
		}
		score := scoreScene(scene)
		if score > bestScore {
			bestScore = score
			bestSceneID = member.SceneID
		}
	}

	if bestSceneID > 0 {
		if err := ds.groupRepo.SetBestScene(groupID, bestSceneID); err != nil {
			return 0, fmt.Errorf("failed to set best scene: %w", err)
		}
	}

	return bestSceneID, nil
}

// ResolveGroup resolves a duplicate group: keeps the best variant, trashes others
func (ds *DuplicateService) ResolveGroup(groupID, bestSceneID uint, mergeMetadata bool) error {
	group, err := ds.groupRepo.GetByIDWithMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	// Validate bestSceneID is a member
	found := false
	var otherSceneIDs []uint
	for _, m := range group.Members {
		if m.SceneID == bestSceneID {
			found = true
		} else {
			otherSceneIDs = append(otherSceneIDs, m.SceneID)
		}
	}
	if !found {
		return fmt.Errorf("scene %d is not a member of group %d", bestSceneID, groupID)
	}

	// Merge metadata if requested
	if mergeMetadata && len(otherSceneIDs) > 0 {
		if err := ds.mergeMetadata(bestSceneID, otherSceneIDs); err != nil {
			ds.logger.Error("Failed to merge metadata",
				zap.Uint("best_scene_id", bestSceneID),
				zap.Error(err),
			)
		}
	}

	// Update is_best flags
	for _, m := range group.Members {
		isBest := m.SceneID == bestSceneID
		if err := ds.groupRepo.UpdateMemberBest(groupID, m.SceneID, isBest); err != nil {
			ds.logger.Error("Failed to update member best flag",
				zap.Uint("scene_id", m.SceneID),
				zap.Error(err),
			)
		}
	}

	// Trash non-best scenes
	for _, sid := range otherSceneIDs {
		if _, err := ds.sceneRepo.MoveToTrash(sid); err != nil {
			ds.logger.Error("Failed to trash scene",
				zap.Uint("scene_id", sid),
				zap.Error(err),
			)
		}
	}

	// Update group status
	now := time.Now()
	if err := ds.groupRepo.SetBestScene(groupID, bestSceneID); err != nil {
		return fmt.Errorf("failed to set best scene: %w", err)
	}
	if err := ds.groupRepo.UpdateStatus(groupID, "resolved", &now); err != nil {
		return fmt.Errorf("failed to update group status: %w", err)
	}

	ds.logger.Info("Resolved duplicate group",
		zap.Uint("group_id", groupID),
		zap.Uint("best_scene_id", bestSceneID),
		zap.Int("trashed_count", len(otherSceneIDs)),
	)

	return nil
}

// DismissGroup marks a group as dismissed (no action taken)
func (ds *DuplicateService) DismissGroup(groupID uint) error {
	return ds.groupRepo.UpdateStatus(groupID, "dismissed", nil)
}

// SetBest updates the best variant for a group
func (ds *DuplicateService) SetBest(groupID, sceneID uint) error {
	group, err := ds.groupRepo.GetByIDWithMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	// Validate sceneID is a member
	found := false
	for _, m := range group.Members {
		if m.SceneID == sceneID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("scene %d is not a member of group %d", sceneID, groupID)
	}

	// Update is_best flags
	for _, m := range group.Members {
		isBest := m.SceneID == sceneID
		if err := ds.groupRepo.UpdateMemberBest(groupID, m.SceneID, isBest); err != nil {
			return fmt.Errorf("failed to update member best: %w", err)
		}
	}

	return ds.groupRepo.SetBestScene(groupID, sceneID)
}

// enrichGroup converts a DuplicateGroup to DuplicateGroupWithScenes by fetching scene details.
// Uses batch fetch including trashed scenes so resolved groups still show all members.
func (ds *DuplicateService) enrichGroup(group *data.DuplicateGroup) (*DuplicateGroupWithScenes, error) {
	if len(group.Members) == 0 {
		return &DuplicateGroupWithScenes{
			ID:          group.ID,
			Status:      group.Status,
			SceneCount:  group.SceneCount,
			BestSceneID: group.BestSceneID,
			Members:     []DuplicateGroupMemberDetail{},
			CreatedAt:   group.CreatedAt,
			UpdatedAt:   group.UpdatedAt,
			ResolvedAt:  group.ResolvedAt,
		}, nil
	}

	// Collect all scene IDs and fetch in one batch (including trashed)
	sceneIDs := make([]uint, len(group.Members))
	for i, m := range group.Members {
		sceneIDs[i] = m.SceneID
	}

	scenes, err := ds.sceneRepo.GetByIDsIncludingTrashed(sceneIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch scenes: %w", err)
	}

	sceneMap := make(map[uint]data.Scene, len(scenes))
	for _, s := range scenes {
		sceneMap[s.ID] = s
	}

	members := make([]DuplicateGroupMemberDetail, 0, len(group.Members))
	for _, m := range group.Members {
		scene, ok := sceneMap[m.SceneID]
		if !ok {
			ds.logger.Warn("Scene not found for group member",
				zap.Uint("scene_id", m.SceneID),
			)
			continue
		}
		members = append(members, DuplicateGroupMemberDetail{
			SceneID:         scene.ID,
			Title:           scene.Title,
			Duration:        scene.Duration,
			Width:           scene.Width,
			Height:          scene.Height,
			VideoCodec:      scene.VideoCodec,
			AudioCodec:      scene.AudioCodec,
			BitRate:         scene.BitRate,
			Size:            scene.Size,
			ThumbnailPath:   scene.ThumbnailPath,
			IsBest:          m.IsBest,
			ConfidenceScore: m.ConfidenceScore,
			MatchType:       m.MatchType,
			IsTrashed:       scene.TrashedAt != nil,
			TrashedAt:       scene.TrashedAt,
		})
	}

	return &DuplicateGroupWithScenes{
		ID:          group.ID,
		Status:      group.Status,
		SceneCount:  group.SceneCount,
		BestSceneID: group.BestSceneID,
		Members:     members,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
		ResolvedAt:  group.ResolvedAt,
	}, nil
}

// mergeMetadata merges tags, actors, view counts, studio, markers, and interactions
// from other scenes into the best scene
func (ds *DuplicateService) mergeMetadata(bestSceneID uint, otherSceneIDs []uint) error {
	// Merge tags
	if ds.tagRepo != nil {
		bestTags, err := ds.tagRepo.GetSceneTags(bestSceneID)
		if err != nil {
			ds.logger.Warn("Failed to get best scene tags", zap.Error(err))
		} else {
			bestTagIDs := make(map[uint]bool)
			for _, t := range bestTags {
				bestTagIDs[t.ID] = true
			}
			for _, sid := range otherSceneIDs {
				otherTags, err := ds.tagRepo.GetSceneTags(sid)
				if err != nil {
					continue
				}
				var newTagIDs []uint
				for _, t := range otherTags {
					if !bestTagIDs[t.ID] {
						newTagIDs = append(newTagIDs, t.ID)
						bestTagIDs[t.ID] = true
					}
				}
				if len(newTagIDs) > 0 {
					if err := ds.tagRepo.BulkAddTagsToScenes([]uint{bestSceneID}, newTagIDs); err != nil {
						ds.logger.Warn("Failed to merge tags from scene",
							zap.Uint("source_scene_id", sid),
							zap.Error(err),
						)
					}
				}
			}
		}
	}

	// Merge actors
	if ds.actorRepo != nil {
		bestActors, err := ds.actorRepo.GetSceneActors(bestSceneID)
		if err != nil {
			ds.logger.Warn("Failed to get best scene actors", zap.Error(err))
		} else {
			bestActorIDs := make(map[uint]bool)
			for _, a := range bestActors {
				bestActorIDs[a.ID] = true
			}
			for _, sid := range otherSceneIDs {
				otherActors, err := ds.actorRepo.GetSceneActors(sid)
				if err != nil {
					continue
				}
				var newActorIDs []uint
				for _, a := range otherActors {
					if !bestActorIDs[a.ID] {
						newActorIDs = append(newActorIDs, a.ID)
						bestActorIDs[a.ID] = true
					}
				}
				if len(newActorIDs) > 0 {
					if err := ds.actorRepo.BulkAddActorsToScenes([]uint{bestSceneID}, newActorIDs); err != nil {
						ds.logger.Warn("Failed to merge actors from scene",
							zap.Uint("source_scene_id", sid),
							zap.Error(err),
						)
					}
				}
			}
		}
	}

	// Merge studio from other scenes: copy to best scene if it has none
	allIDs := append([]uint{bestSceneID}, otherSceneIDs...)
	allScenes, err := ds.sceneRepo.GetByIDsIncludingTrashed(allIDs)
	if err != nil {
		ds.logger.Warn("Failed to fetch scenes for metadata merge", zap.Error(err))
	} else {
		sceneMap := make(map[uint]data.Scene, len(allScenes))
		for _, s := range allScenes {
			sceneMap[s.ID] = s
		}

		bestScene := sceneMap[bestSceneID]

		// Copy studio if best scene has none but a duplicate does
		if bestScene.StudioID == nil {
			for _, sid := range otherSceneIDs {
				other, ok := sceneMap[sid]
				if ok && other.StudioID != nil {
					if err := ds.sceneRepo.UpdateSceneMetadata(bestSceneID, bestScene.Title, bestScene.Description, other.Studio, bestScene.ReleaseDate, bestScene.PornDBSceneID); err != nil {
						ds.logger.Warn("Failed to copy studio from duplicate",
							zap.Uint("source_scene_id", sid),
							zap.Error(err),
						)
					}
					break
				}
			}
		}
	}

	// Merge markers
	if ds.markerRepo != nil {
		for _, sid := range otherSceneIDs {
			if err := ds.markerRepo.ReassignMarkersToScene(sid, bestSceneID); err != nil {
				ds.logger.Warn("Failed to reassign markers from scene",
					zap.Uint("source_scene_id", sid),
					zap.Uint("target_scene_id", bestSceneID),
					zap.Error(err),
				)
			}
		}
	}

	// Merge interactions (ratings, likes, jizzed)
	if ds.interactionRepo != nil {
		for _, sid := range otherSceneIDs {
			if err := ds.interactionRepo.ReassignInteractionsToScene(sid, bestSceneID); err != nil {
				ds.logger.Warn("Failed to reassign interactions from scene",
					zap.Uint("source_scene_id", sid),
					zap.Uint("target_scene_id", bestSceneID),
					zap.Error(err),
				)
			}
		}
	}

	ds.logger.Info("Merged metadata into best scene",
		zap.Uint("best_scene_id", bestSceneID),
		zap.Int("source_count", len(otherSceneIDs)),
	)

	return nil
}

