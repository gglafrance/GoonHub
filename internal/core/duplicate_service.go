package core

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"path/filepath"
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
	FileName        string     `json:"file_name"`
	// Matching segment in this scene and the corresponding segment in RefSceneID (seconds)
	MatchStart *float64 `json:"match_start,omitempty"`
	MatchEnd   *float64 `json:"match_end,omitempty"`
	RefSceneID *uint    `json:"ref_scene_id,omitempty"`
	RefStart   *float64 `json:"ref_start,omitempty"`
	RefEnd     *float64 `json:"ref_end,omitempty"`
	// DeleteAfter is when the scene (not kept at resolution) will be permanently deleted
	DeleteAfter *time.Time `json:"delete_after,omitempty"`
}

// BulkResolveFailure reports a group that could not be resolved by ResolveGroups.
type BulkResolveFailure struct {
	GroupID uint   `json:"group_id"`
	Error   string `json:"error"`
}

// BulkResolveResult is the outcome of ResolveGroups.
type BulkResolveResult struct {
	Resolved []uint               `json:"resolved"`
	Failed   []BulkResolveFailure `json:"failed"`
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
	sceneService    *SceneService
	configRepo      data.DuplicationConfigRepository
	logger          *zap.Logger
	cancelCleanup   context.CancelFunc
}

// deletionCleanupInterval is how often scheduled deletions of resolved duplicates are applied.
const deletionCleanupInterval = 10 * time.Minute

// deletionCleanupBatch caps the deletions applied per cleanup run.
const deletionCleanupBatch = 200

// SetConfigRepo provides the duplication config (deletion of the scenes not kept at resolution).
func (ds *DuplicateService) SetConfigRepo(repo data.DuplicationConfigRepository) {
	ds.configRepo = repo
}

// NewDuplicateService creates a new DuplicateService
func NewDuplicateService(
	groupRepo data.DuplicateGroupRepository,
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	markerRepo data.MarkerRepository,
	interactionRepo data.InteractionRepository,
	sceneService *SceneService,
	logger *zap.Logger,
) *DuplicateService {
	return &DuplicateService{
		groupRepo:       groupRepo,
		sceneRepo:       sceneRepo,
		tagRepo:         tagRepo,
		actorRepo:       actorRepo,
		markerRepo:      markerRepo,
		interactionRepo: interactionRepo,
		sceneService:    sceneService,
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
	var best *data.Scene

	for _, member := range group.Members {
		scene, err := ds.sceneRepo.GetByID(member.SceneID)
		if err != nil {
			continue
		}
		if best == nil || betterSceneBy(scene, best, ds.bestCriteria()) {
			best = scene
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
			return fmt.Errorf("failed to merge metadata into scene %d: %w", bestSceneID, err)
		}
	}

	// Update is_best flags
	for _, m := range group.Members {
		isBest := m.SceneID == bestSceneID
		if err := ds.groupRepo.UpdateMemberBest(groupID, m.SceneID, isBest); err != nil {
			return fmt.Errorf("failed to update best flag for scene %d: %w", m.SceneID, err)
		}
	}

	// Trash non-best scenes
	for _, sid := range otherSceneIDs {
		if _, err := ds.sceneService.MoveSceneToTrash(sid); err != nil {
			return fmt.Errorf("failed to trash scene %d: %w", sid, err)
		}
	}

	// Schedule the permanent deletion of the trashed scenes when configured
	if deleteAfter := ds.scheduledDeletionTime(); deleteAfter != nil && len(otherSceneIDs) > 0 {
		if err := ds.groupRepo.SetMemberDeleteAfter(groupID, otherSceneIDs, deleteAfter); err != nil {
			return fmt.Errorf("failed to schedule deletion of resolved duplicates: %w", err)
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

// scheduledDeletionTime returns when the scenes not kept at resolution must be deleted, or nil
// when deletion of resolved duplicates is disabled.
func (ds *DuplicateService) scheduledDeletionTime() *time.Time {
	if ds.configRepo == nil {
		return nil
	}
	cfg, err := ds.configRepo.Get()
	if err != nil {
		ds.logger.Warn("Failed to read duplication config, not scheduling deletion", zap.Error(err))
		return nil
	}
	if cfg == nil || !cfg.DeleteResolvedFiles {
		return nil
	}
	at := time.Now().Add(time.Duration(max(cfg.DeletionDelayHours, 0)) * time.Hour)
	return &at
}

// bestCriteria returns the configured best variant selection criteria.
func (ds *DuplicateService) bestCriteria() []string {
	if ds.configRepo == nil {
		return DefaultBestCriteria
	}
	cfg, err := ds.configRepo.Get()
	if err != nil {
		ds.logger.Warn("Failed to read duplication config, using default best criteria", zap.Error(err))
		return DefaultBestCriteria
	}
	return bestCriteriaFromConfig(cfg)
}

// bestMemberScene returns the best variant to keep among the group's non-trashed members.
func (ds *DuplicateService) bestMemberScene(group *data.DuplicateGroup) (uint, error) {
	sceneIDs := make([]uint, len(group.Members))
	for i, m := range group.Members {
		sceneIDs[i] = m.SceneID
	}
	scenes, err := ds.sceneRepo.GetByIDsIncludingTrashed(sceneIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch group scenes: %w", err)
	}
	criteria := ds.bestCriteria()
	var best *data.Scene
	for i := range scenes {
		if scenes[i].TrashedAt != nil {
			continue
		}
		if best == nil || betterSceneBy(&scenes[i], best, criteria) {
			best = &scenes[i]
		}
	}
	if best == nil {
		return 0, fmt.Errorf("group %d has no scene left to keep", group.ID)
	}
	return best.ID, nil
}

// AutoSelectBest marks, in each unresolved group, the best variant according to the configured
// criteria (default: highest resolution, then longest duration, then highest bitrate, then
// smallest file). Returns group ID -> scene ID.
func (ds *DuplicateService) AutoSelectBest(groupIDs []uint) (map[uint]uint, error) {
	selected := make(map[uint]uint, len(groupIDs))
	for _, groupID := range groupIDs {
		group, err := ds.groupRepo.GetByIDWithMembers(groupID)
		if err != nil {
			return selected, fmt.Errorf("failed to get group %d: %w", groupID, err)
		}
		if group.Status != "unresolved" {
			continue
		}
		bestID, err := ds.bestMemberScene(group)
		if err != nil {
			return selected, err
		}
		if err := ds.SetBest(groupID, bestID); err != nil {
			return selected, err
		}
		selected[groupID] = bestID
	}
	return selected, nil
}

// ResolveGroups resolves several unresolved groups. With autoSelect the best variant is chosen
// automatically, otherwise the group's current best (or the automatic choice when none is set).
// A failing group does not stop the others.
func (ds *DuplicateService) ResolveGroups(groupIDs []uint, autoSelect, mergeMetadata bool) BulkResolveResult {
	result := BulkResolveResult{Resolved: []uint{}, Failed: []BulkResolveFailure{}}
	fail := func(groupID uint, err error) {
		result.Failed = append(result.Failed, BulkResolveFailure{GroupID: groupID, Error: err.Error()})
	}
	for _, groupID := range groupIDs {
		group, err := ds.groupRepo.GetByIDWithMembers(groupID)
		if err != nil {
			fail(groupID, fmt.Errorf("failed to get group: %w", err))
			continue
		}
		if group.Status != "unresolved" {
			fail(groupID, fmt.Errorf("group is %s", group.Status))
			continue
		}
		var bestID uint
		if !autoSelect && group.BestSceneID != nil {
			bestID = *group.BestSceneID
		} else if bestID, err = ds.bestMemberScene(group); err != nil {
			fail(groupID, err)
			continue
		}
		if err := ds.ResolveGroup(groupID, bestID, mergeMetadata); err != nil {
			fail(groupID, err)
			continue
		}
		result.Resolved = append(result.Resolved, groupID)
	}
	ds.logger.Info("Bulk resolved duplicate groups",
		zap.Int("resolved", len(result.Resolved)),
		zap.Int("failed", len(result.Failed)),
	)
	return result
}

// CancelDeletion cancels the scheduled deletion of a scene not kept at resolution.
func (ds *DuplicateService) CancelDeletion(groupID, sceneID uint) error {
	return ds.groupRepo.SetMemberDeleteAfter(groupID, []uint{sceneID}, nil)
}

// RunDeletionCleanup permanently deletes the scenes (and their files) whose scheduled deletion
// time has passed. A scene restored from the trash meanwhile is kept and unscheduled.
func (ds *DuplicateService) RunDeletionCleanup() (int, error) {
	members, err := ds.groupRepo.ListMembersDueForDeletion(time.Now(), deletionCleanupBatch)
	if err != nil {
		return 0, fmt.Errorf("failed to list scheduled deletions: %w", err)
	}
	deleted := 0
	for _, m := range members {
		scene, err := ds.sceneRepo.GetByIDIncludingTrashed(m.SceneID)
		if err != nil {
			ds.logger.Warn("Scheduled deletion: scene not found", zap.Uint("scene_id", m.SceneID), zap.Error(err))
			continue
		}
		if scene.TrashedAt == nil {
			// Restored from the trash meanwhile: keep it
			if err := ds.groupRepo.SetMemberDeleteAfter(m.GroupID, []uint{m.SceneID}, nil); err != nil {
				ds.logger.Warn("Failed to unschedule restored scene", zap.Uint("scene_id", m.SceneID), zap.Error(err))
			}
			continue
		}
		if err := ds.sceneService.HardDeleteScene(m.SceneID); err != nil {
			ds.logger.Warn("Scheduled deletion failed", zap.Uint("scene_id", m.SceneID), zap.Error(err))
			continue
		}
		deleted++
		ds.logger.Info("Deleted resolved duplicate",
			zap.Uint("scene_id", m.SceneID),
			zap.Uint("group_id", m.GroupID),
			zap.String("path", scene.StoredPath),
		)
	}
	return deleted, nil
}

// StartDeletionCleanupTicker applies scheduled deletions now and then periodically.
func (ds *DuplicateService) StartDeletionCleanupTicker() {
	ctx, cancel := context.WithCancel(context.Background())
	ds.cancelCleanup = cancel
	run := func() {
		if _, err := ds.RunDeletionCleanup(); err != nil {
			ds.logger.Warn("Duplicate deletion cleanup failed", zap.Error(err))
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(deletionCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// StopDeletionCleanupTicker stops the periodic cleanup.
func (ds *DuplicateService) StopDeletionCleanupTicker() {
	if ds.cancelCleanup != nil {
		ds.cancelCleanup()
	}
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
			FileName:        filepath.Base(scene.StoredPath),
			MatchStart:      m.MatchStart,
			MatchEnd:        m.MatchEnd,
			RefSceneID:      m.RefSceneID,
			RefStart:        m.RefStart,
			RefEnd:          m.RefEnd,
			DeleteAfter:     m.DeleteAfter,
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
	// Merge tags.
	if ds.tagRepo != nil {
		bestTags, err := ds.tagRepo.GetSceneTags(bestSceneID)
		if err != nil {
			return fmt.Errorf("failed to get tags for scene %d: %w", bestSceneID, err)
		}
		bestTagIDs := make(map[uint]bool)
		for _, tag := range bestTags {
			bestTagIDs[tag.ID] = true
		}
		for _, sid := range otherSceneIDs {
			tags, err := ds.tagRepo.GetSceneTags(sid)
			if err != nil {
				return fmt.Errorf("failed to get tags for scene %d: %w", sid, err)
			}
			var newTagIDs []uint
			for _, tag := range tags {
				if !bestTagIDs[tag.ID] {
					newTagIDs = append(newTagIDs, tag.ID)
					bestTagIDs[tag.ID] = true
				}
			}
			if len(newTagIDs) > 0 {
				if err := ds.tagRepo.BulkAddTagsToScenes([]uint{bestSceneID}, newTagIDs); err != nil {
					return fmt.Errorf("failed to merge tags from scene %d: %w", sid, err)
				}
			}
		}
	}

	// Merge actors.
	if ds.actorRepo != nil {
		bestActors, err := ds.actorRepo.GetSceneActors(bestSceneID)
		if err != nil {
			return fmt.Errorf("failed to get actors for scene %d: %w", bestSceneID, err)
		}
		bestActorIDs := make(map[uint]bool)
		for _, actor := range bestActors {
			bestActorIDs[actor.ID] = true
		}
		for _, sid := range otherSceneIDs {
			actors, err := ds.actorRepo.GetSceneActors(sid)
			if err != nil {
				return fmt.Errorf("failed to get actors for scene %d: %w", sid, err)
			}
			var newActorIDs []uint
			for _, actor := range actors {
				if !bestActorIDs[actor.ID] {
					newActorIDs = append(newActorIDs, actor.ID)
					bestActorIDs[actor.ID] = true
				}
			}
			if len(newActorIDs) > 0 {
				if err := ds.actorRepo.BulkAddActorsToScenes([]uint{bestSceneID}, newActorIDs); err != nil {
					return fmt.Errorf("failed to merge actors from scene %d: %w", sid, err)
				}
			}
		}
	}

	// Copy a studio only when the best scene has none.
	allIDs := append([]uint{bestSceneID}, otherSceneIDs...)
	scenes, err := ds.sceneRepo.GetByIDsIncludingTrashed(allIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch scenes for metadata merge: %w", err)
	}
	sceneMap := make(map[uint]data.Scene, len(scenes))
	for _, scene := range scenes {
		sceneMap[scene.ID] = scene
	}
	bestScene, ok := sceneMap[bestSceneID]
	if !ok {
		return fmt.Errorf("best scene %d not found", bestSceneID)
	}
	for _, sid := range otherSceneIDs {
		if _, ok := sceneMap[sid]; !ok {
			return fmt.Errorf("source scene %d not found", sid)
		}
	}
	if bestScene.StudioID == nil {
		for _, sid := range otherSceneIDs {
			other := sceneMap[sid]
			if other.StudioID != nil {
				if err := ds.sceneRepo.UpdateSceneMetadata(bestSceneID, bestScene.Title, bestScene.Description, other.Studio, bestScene.ReleaseDate, bestScene.PornDBSceneID); err != nil {
					return fmt.Errorf("failed to copy studio from scene %d: %w", sid, err)
				}
				break
			}
		}
	}

	if ds.markerRepo != nil {
		for _, sid := range otherSceneIDs {
			if err := ds.markerRepo.ReassignMarkersToScene(sid, bestSceneID); err != nil {
				return fmt.Errorf("failed to reassign markers from scene %d: %w", sid, err)
			}
		}
	}
	if ds.interactionRepo != nil {
		for _, sid := range otherSceneIDs {
			if err := ds.interactionRepo.ReassignInteractionsToScene(sid, bestSceneID); err != nil {
				return fmt.Errorf("failed to reassign interactions from scene %d: %w", sid, err)
			}
		}
	}

	ds.logger.Info("Merged metadata into best scene",
		zap.Uint("best_scene_id", bestSceneID),
		zap.Int("source_count", len(otherSceneIDs)),
	)
	return nil
}
