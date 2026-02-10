package core

import (
	"encoding/json"
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/pkg/fingerprint"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// RescanStatus tracks the progress of a library-wide fingerprint rescan.
type RescanStatus struct {
	Running   bool `json:"running"`
	Total     int  `json:"total"`
	Completed int  `json:"completed"`
	Matched   int  `json:"matched"`
}

// DuplicateDetectionService orchestrates duplicate detection using perceptual hashing.
type DuplicateDetectionService struct {
	fingerprintRepo data.FingerprintRepository
	duplicateRepo   data.DuplicateRepository
	duplicateConfig data.DuplicateConfigRepository
	sceneRepo       data.SceneRepository
	bloomManager    *BloomFilterManager
	eventBus        *EventBus
	logger          *zap.Logger

	rescanStatus atomic.Value // *RescanStatus
	rescanMu     sync.Mutex
}

func NewDuplicateDetectionService(
	fingerprintRepo data.FingerprintRepository,
	duplicateRepo data.DuplicateRepository,
	duplicateConfig data.DuplicateConfigRepository,
	sceneRepo data.SceneRepository,
	bloomManager *BloomFilterManager,
	eventBus *EventBus,
	logger *zap.Logger,
) *DuplicateDetectionService {
	svc := &DuplicateDetectionService{
		fingerprintRepo: fingerprintRepo,
		duplicateRepo:   duplicateRepo,
		duplicateConfig: duplicateConfig,
		sceneRepo:       sceneRepo,
		bloomManager:    bloomManager,
		eventBus:        eventBus,
		logger:          logger.With(zap.String("component", "duplicate_detection")),
	}
	svc.rescanStatus.Store(&RescanStatus{})
	return svc
}

// GetConfig returns the current duplicate detection configuration.
func (s *DuplicateDetectionService) GetConfig() (*data.DuplicateConfigRecord, error) {
	cfg, err := s.duplicateConfig.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get duplicate config: %w", err)
	}
	if cfg == nil {
		return &data.DuplicateConfigRecord{
			ID:              1,
			Enabled:         false,
			CheckOnUpload:   true,
			MatchThreshold:  80,
			HammingDistance:  8,
			SampleInterval:  2,
			DuplicateAction: "flag",
		}, nil
	}
	return cfg, nil
}

// UpdateConfig updates the duplicate detection configuration.
func (s *DuplicateDetectionService) UpdateConfig(cfg *data.DuplicateConfigRecord) error {
	return s.duplicateConfig.Upsert(cfg)
}

// CheckForDuplicates checks a newly fingerprinted scene against all existing fingerprints.
// Returns true if a duplicate match was found.
func (s *DuplicateDetectionService) CheckForDuplicates(sceneID uint) (bool, error) {
	s.logger.Debug("Starting duplicate check", zap.Uint("scene_id", sceneID))

	cfg, err := s.GetConfig()
	if err != nil {
		s.logger.Error("Failed to get duplicate config", zap.Uint("scene_id", sceneID), zap.Error(err))
		return false, err
	}
	if !cfg.Enabled {
		s.logger.Debug("Duplicate detection disabled, skipping", zap.Uint("scene_id", sceneID))
		return false, nil
	}

	// Load scene's hashes
	sceneFingerprints, err := s.fingerprintRepo.GetBySceneID(sceneID)
	if err != nil {
		return false, fmt.Errorf("failed to load fingerprints for scene %d: %w", sceneID, err)
	}
	if len(sceneFingerprints) == 0 {
		s.logger.Debug("No fingerprints found for scene", zap.Uint("scene_id", sceneID))
		return false, nil
	}

	s.logger.Debug("Loaded fingerprints for scene",
		zap.Uint("scene_id", sceneID),
		zap.Int("fingerprint_count", len(sceneFingerprints)),
	)

	sourceHashes := make([]uint64, len(sceneFingerprints))
	for i, fp := range sceneFingerprints {
		sourceHashes[i] = uint64(fp.HashValue)
	}

	// Bloom filter pre-screen
	s.logger.Debug("Checking bloom filter", zap.Uint("scene_id", sceneID))
	if !s.bloomManager.MayContainAny(sourceHashes) {
		s.logger.Debug("Bloom filter indicates no potential matches, adding hashes", zap.Uint("scene_id", sceneID))
		s.bloomManager.AddHashes(sourceHashes)
		return false, nil
	}
	s.logger.Debug("Bloom filter indicates potential matches, proceeding with full check", zap.Uint("scene_id", sceneID))

	// Get all fingerprinted scene IDs (excluding current)
	candidateIDs, err := s.fingerprintRepo.GetFingerprintedSceneIDs()
	if err != nil {
		return false, fmt.Errorf("failed to get fingerprinted scene IDs: %w", err)
	}

	s.logger.Debug("Loaded candidate scene IDs",
		zap.Uint("scene_id", sceneID),
		zap.Int("candidate_count", len(candidateIDs)),
	)

	matchThreshold := float64(cfg.MatchThreshold)
	hammingDist := cfg.HammingDistance
	matched := false

	for _, candidateID := range candidateIDs {
		if candidateID == sceneID {
			continue
		}

		s.logger.Debug("Checking candidate scene",
			zap.Uint("source_scene_id", sceneID),
			zap.Uint("candidate_id", candidateID),
		)

		candidateFingerprints, err := s.fingerprintRepo.GetBySceneID(candidateID)
		if err != nil {
			s.logger.Warn("Failed to load candidate fingerprints",
				zap.Uint("candidate_id", candidateID),
				zap.Error(err),
			)
			continue
		}

		if len(candidateFingerprints) == 0 {
			s.logger.Debug("Candidate has no fingerprints, skipping",
				zap.Uint("candidate_id", candidateID),
			)
			continue
		}

		targetHashes := make([]uint64, len(candidateFingerprints))
		for i, fp := range candidateFingerprints {
			targetHashes[i] = uint64(fp.HashValue)
		}

		s.logger.Debug("Comparing fingerprints",
			zap.Uint("source_scene_id", sceneID),
			zap.Uint("candidate_id", candidateID),
			zap.Int("source_hash_count", len(sourceHashes)),
			zap.Int("target_hash_count", len(targetHashes)),
			zap.Int("hamming_distance", hammingDist),
			zap.Float64("match_threshold", matchThreshold),
		)

		result := fingerprint.FindMatches(sourceHashes, targetHashes, hammingDist, matchThreshold)
		if result == nil {
			s.logger.Debug("No match found with candidate",
				zap.Uint("source_scene_id", sceneID),
				zap.Uint("candidate_id", candidateID),
			)
			continue
		}

		s.logger.Info("Duplicate detected",
			zap.Uint("source_scene_id", sceneID),
			zap.Uint("target_scene_id", candidateID),
			zap.Float64("match_percent", result.MatchPercent),
			zap.Int("frame_offset", result.FrameOffset),
		)

		// Check if candidate is already in a group
		existingGroup, err := s.duplicateRepo.GetGroupForScene(candidateID)
		if err != nil {
			s.logger.Error("Failed to check existing group for candidate",
				zap.Uint("candidate_id", candidateID),
				zap.Error(err),
			)
			continue
		}

		if existingGroup != nil {
			s.logger.Debug("Adding scene to existing duplicate group",
				zap.Uint("scene_id", sceneID),
				zap.Uint("group_id", existingGroup.ID),
			)

			// Add to existing group
			if err := s.duplicateRepo.AddMember(&data.DuplicateGroupMember{
				GroupID:         existingGroup.ID,
				SceneID:         sceneID,
				MatchPercentage: result.MatchPercent,
				FrameOffset:     result.FrameOffset,
			}); err != nil {
				s.logger.Error("Failed to add scene to existing group",
					zap.Uint("scene_id", sceneID),
					zap.Uint("group_id", existingGroup.ID),
					zap.Error(err),
				)
				continue
			}
			if err := s.sceneRepo.UpdateDuplicateGroupID(sceneID, &existingGroup.ID); err != nil {
				s.logger.Error("Failed to link scene to existing group",
					zap.Uint("scene_id", sceneID),
					zap.Uint("group_id", existingGroup.ID),
					zap.Error(err),
				)
			}

			s.logger.Debug("Successfully added scene to existing group",
				zap.Uint("scene_id", sceneID),
				zap.Uint("group_id", existingGroup.ID),
			)

			s.eventBus.Publish(SceneEvent{
				Type:    "scene:duplicate_detected",
				SceneID: sceneID,
				Data: map[string]any{
					"group_id": existingGroup.ID,
				},
			})
		} else {
			s.logger.Debug("Creating new duplicate group",
				zap.Uint("source_scene_id", sceneID),
				zap.Uint("candidate_id", candidateID),
			)

			// Create new group
			group := &data.DuplicateGroup{Status: "pending"}
			if err := s.duplicateRepo.CreateGroup(group); err != nil {
				s.logger.Error("Failed to create duplicate group", zap.Error(err))
				continue
			}

			s.logger.Debug("Created duplicate group", zap.Uint("group_id", group.ID))

			// Add both scenes as members
			if err := s.duplicateRepo.AddMember(&data.DuplicateGroupMember{
				GroupID:         group.ID,
				SceneID:         candidateID,
				MatchPercentage: 100, // reference scene
				FrameOffset:     0,
			}); err != nil {
				s.logger.Error("Failed to add candidate to new group",
					zap.Uint("scene_id", candidateID),
					zap.Uint("group_id", group.ID),
					zap.Error(err),
				)
				continue
			}
			if err := s.duplicateRepo.AddMember(&data.DuplicateGroupMember{
				GroupID:         group.ID,
				SceneID:         sceneID,
				MatchPercentage: result.MatchPercent,
				FrameOffset:     result.FrameOffset,
			}); err != nil {
				s.logger.Error("Failed to add source to new group",
					zap.Uint("scene_id", sceneID),
					zap.Uint("group_id", group.ID),
					zap.Error(err),
				)
				continue
			}

			if err := s.sceneRepo.UpdateDuplicateGroupID(candidateID, &group.ID); err != nil {
				s.logger.Error("Failed to link candidate to group",
					zap.Uint("scene_id", candidateID),
					zap.Uint("group_id", group.ID),
					zap.Error(err),
				)
			}
			if err := s.sceneRepo.UpdateDuplicateGroupID(sceneID, &group.ID); err != nil {
				s.logger.Error("Failed to link source to group",
					zap.Uint("scene_id", sceneID),
					zap.Uint("group_id", group.ID),
					zap.Error(err),
				)
			}

			s.logger.Debug("Successfully created and populated duplicate group",
				zap.Uint("group_id", group.ID),
				zap.Uint("candidate_id", candidateID),
				zap.Uint("source_scene_id", sceneID),
			)

			s.eventBus.Publish(SceneEvent{
				Type:    "scene:duplicate_detected",
				SceneID: sceneID,
				Data: map[string]any{
					"group_id": group.ID,
				},
			})
		}

		matched = true
		s.logger.Debug("Duplicate match processed, stopping further checks", zap.Uint("scene_id", sceneID))
		// Only need to find one duplicate match
		break
	}

	// Add to bloom filter
	s.logger.Debug("Adding hashes to bloom filter", zap.Uint("scene_id", sceneID))
	s.bloomManager.AddHashes(sourceHashes)

	s.logger.Debug("Duplicate check completed",
		zap.Uint("scene_id", sceneID),
		zap.Bool("matched", matched),
	)

	return matched, nil
}

// DetermineWinner evaluates scenes in a duplicate group and picks the best one.
func (s *DuplicateDetectionService) DetermineWinner(groupID uint) (uint, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return 0, err
	}

	members, err := s.duplicateRepo.GetMembersForGroup(groupID)
	if err != nil {
		return 0, fmt.Errorf("failed to get group members: %w", err)
	}
	if len(members) == 0 {
		return 0, fmt.Errorf("no members in group %d", groupID)
	}

	// Load scenes for comparison
	sceneIDs := make([]uint, len(members))
	for i, m := range members {
		sceneIDs[i] = m.SceneID
	}
	scenes, err := s.sceneRepo.GetByIDs(sceneIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to load scenes: %w", err)
	}
	if len(scenes) == 0 {
		return 0, fmt.Errorf("no scenes found for group %d", groupID)
	}

	// Parse rules
	var rules []string
	if err := json.Unmarshal(cfg.KeepBestRules, &rules); err != nil {
		rules = []string{"duration", "resolution", "codec", "bitrate"}
	}
	var enabledMap map[string]bool
	if err := json.Unmarshal(cfg.KeepBestEnabled, &enabledMap); err != nil {
		enabledMap = map[string]bool{"duration": true, "resolution": true, "codec": true, "bitrate": true}
	}
	var codecPref []string
	if err := json.Unmarshal(cfg.CodecPreference, &codecPref); err != nil {
		codecPref = []string{"h265", "hevc", "av1", "vp9", "h264"}
	}

	// Start with all scene indices as candidates
	candidates := make([]int, len(scenes))
	for i := range scenes {
		candidates[i] = i
	}

	for _, rule := range rules {
		if !enabledMap[rule] || len(candidates) <= 1 {
			continue
		}

		// Find the best value among candidates, then filter to only those matching the best
		switch rule {
		case "duration":
			best := scenes[candidates[0]].Duration
			for _, idx := range candidates[1:] {
				if scenes[idx].Duration > best {
					best = scenes[idx].Duration
				}
			}
			filtered := candidates[:0]
			for _, idx := range candidates {
				if scenes[idx].Duration == best {
					filtered = append(filtered, idx)
				}
			}
			candidates = filtered

		case "resolution":
			bestRes := scenes[candidates[0]].Width * scenes[candidates[0]].Height
			for _, idx := range candidates[1:] {
				res := scenes[idx].Width * scenes[idx].Height
				if res > bestRes {
					bestRes = res
				}
			}
			filtered := candidates[:0]
			for _, idx := range candidates {
				if scenes[idx].Width*scenes[idx].Height == bestRes {
					filtered = append(filtered, idx)
				}
			}
			candidates = filtered

		case "codec":
			bestRank := codecRank(scenes[candidates[0]].VideoCodec, codecPref)
			for _, idx := range candidates[1:] {
				rank := codecRank(scenes[idx].VideoCodec, codecPref)
				if rank < bestRank {
					bestRank = rank
				}
			}
			filtered := candidates[:0]
			for _, idx := range candidates {
				if codecRank(scenes[idx].VideoCodec, codecPref) == bestRank {
					filtered = append(filtered, idx)
				}
			}
			candidates = filtered

		case "bitrate":
			best := scenes[candidates[0]].BitRate
			for _, idx := range candidates[1:] {
				if scenes[idx].BitRate > best {
					best = scenes[idx].BitRate
				}
			}
			filtered := candidates[:0]
			for _, idx := range candidates {
				if scenes[idx].BitRate == best {
					filtered = append(filtered, idx)
				}
			}
			candidates = filtered
		}
	}

	return scenes[candidates[0]].ID, nil
}

func codecRank(codec string, preference []string) int {
	lc := strings.ToLower(codec)
	for i, pref := range preference {
		if strings.Contains(lc, strings.ToLower(pref)) {
			return i
		}
	}
	return len(preference) // unknown codec ranks last
}

// ResolveDuplicateGroup auto-resolves a group using keep-best rules.
func (s *DuplicateDetectionService) ResolveDuplicateGroup(groupID uint) error {
	winnerID, err := s.DetermineWinner(groupID)
	if err != nil {
		return err
	}

	return s.SetWinner(groupID, winnerID)
}

// SetWinner manually sets the winner for a duplicate group.
func (s *DuplicateDetectionService) SetWinner(groupID, winnerSceneID uint) error {
	// Clear existing winners
	if err := s.duplicateRepo.ClearMemberWinners(groupID); err != nil {
		return fmt.Errorf("failed to clear member winners: %w", err)
	}

	// Set new winner
	if err := s.duplicateRepo.SetMemberWinner(groupID, winnerSceneID); err != nil {
		return fmt.Errorf("failed to set member winner: %w", err)
	}
	if err := s.duplicateRepo.SetGroupWinner(groupID, winnerSceneID); err != nil {
		return fmt.Errorf("failed to set group winner: %w", err)
	}
	if err := s.duplicateRepo.UpdateGroupStatus(groupID, "resolved"); err != nil {
		return fmt.Errorf("failed to update group status: %w", err)
	}

	s.eventBus.Publish(SceneEvent{
		Type:    "scene:duplicate_resolved",
		SceneID: winnerSceneID,
		Data: map[string]any{
			"group_id": groupID,
		},
	})

	return nil
}

// DismissGroup marks a duplicate group as dismissed.
func (s *DuplicateDetectionService) DismissGroup(groupID uint) error {
	return s.duplicateRepo.UpdateGroupStatus(groupID, "dismissed")
}

// DeleteGroup permanently removes a duplicate group.
func (s *DuplicateDetectionService) DeleteGroup(groupID uint) error {
	// Clear duplicate_group_id from member scenes
	members, err := s.duplicateRepo.GetMembersForGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get members: %w", err)
	}
	for _, m := range members {
		if err := s.sceneRepo.UpdateDuplicateGroupID(m.SceneID, nil); err != nil {
			s.logger.Error("Failed to clear duplicate group ID",
				zap.Uint("scene_id", m.SceneID),
				zap.Uint("group_id", groupID),
				zap.Error(err),
			)
		}
	}

	return s.duplicateRepo.DeleteGroup(groupID)
}

// ListGroups returns paginated duplicate groups.
func (s *DuplicateDetectionService) ListGroups(page, limit int, status string) ([]data.DuplicateGroup, int64, error) {
	return s.duplicateRepo.ListGroups(page, limit, status)
}

// GetGroup returns a duplicate group with its members.
func (s *DuplicateDetectionService) GetGroup(groupID uint) (*data.DuplicateGroup, error) {
	return s.duplicateRepo.GetGroupByIDWithMembers(groupID)
}

// GetRescanStatus returns the current rescan progress.
func (s *DuplicateDetectionService) GetRescanStatus() *RescanStatus {
	return s.rescanStatus.Load().(*RescanStatus)
}

// StartRescan triggers a full library duplicate scan in the background.
func (s *DuplicateDetectionService) StartRescan() error {
	s.rescanMu.Lock()
	status := s.GetRescanStatus()
	if status.Running {
		s.rescanMu.Unlock()
		s.logger.Debug("Rescan already running, rejecting new request")
		return apperrors.ErrRescanAlreadyRunning
	}
	s.rescanStatus.Store(&RescanStatus{Running: true})
	s.rescanMu.Unlock()

	s.logger.Info("Starting duplicate rescan")

	go func() {
		defer func() {
			st := s.GetRescanStatus()
			s.rescanStatus.Store(&RescanStatus{
				Running:   false,
				Total:     st.Total,
				Completed: st.Completed,
				Matched:   st.Matched,
			})
			s.logger.Debug("Rescan goroutine completed, status updated",
				zap.Int("total", st.Total),
				zap.Int("completed", st.Completed),
				zap.Int("matched", st.Matched),
			)
		}()

		// Rebuild bloom filter first
		s.logger.Debug("Rebuilding bloom filter")
		if err := s.bloomManager.Rebuild(); err != nil {
			s.logger.Error("Failed to rebuild bloom filter during rescan", zap.Error(err))
			return
		}
		s.logger.Debug("Bloom filter rebuilt successfully")

		// Get all fingerprinted scene IDs
		s.logger.Debug("Fetching fingerprinted scene IDs")
		sceneIDs, err := s.fingerprintRepo.GetFingerprintedSceneIDs()
		if err != nil {
			s.logger.Error("Failed to get fingerprinted scene IDs", zap.Error(err))
			return
		}

		s.logger.Info("Fingerprinted scenes loaded", zap.Int("count", len(sceneIDs)))

		s.rescanStatus.Store(&RescanStatus{
			Running: true,
			Total:   len(sceneIDs),
		})

		matched := 0
		for i, sceneID := range sceneIDs {
			s.logger.Debug("Checking scene for duplicates",
				zap.Uint("scene_id", sceneID),
				zap.Int("progress", i+1),
				zap.Int("total", len(sceneIDs)),
			)

			found, err := s.CheckForDuplicates(sceneID)
			if err != nil {
				s.logger.Warn("Rescan check failed for scene",
					zap.Uint("scene_id", sceneID),
					zap.Error(err),
				)
			}
			if found {
				matched++
				s.logger.Debug("Duplicate match found",
					zap.Uint("scene_id", sceneID),
					zap.Int("matched_count", matched),
				)
			}

			s.rescanStatus.Store(&RescanStatus{
				Running:   true,
				Total:     len(sceneIDs),
				Completed: i + 1,
				Matched:   matched,
			})
		}

		s.logger.Info("Rescan completed",
			zap.Int("total", len(sceneIDs)),
			zap.Int("matched", matched),
		)
	}()

	return nil
}

// CountPendingGroups returns the count of pending duplicate groups.
func (s *DuplicateDetectionService) CountPendingGroups() (int64, error) {
	return s.duplicateRepo.CountPendingGroups()
}
