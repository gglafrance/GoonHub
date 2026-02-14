package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"goonhub/internal/core/matching"
	"goonhub/internal/data"
	chClient "goonhub/internal/infrastructure/clickhouse"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MatchingService orchestrates fingerprint matching and duplicate group management
type MatchingService struct {
	clickhouse    *chClient.Client
	sceneRepo     data.SceneRepository
	dupGroupRepo  data.DuplicateGroupRepository
	dupConfigRepo data.DuplicationConfigRepository
	logger        *zap.Logger
	processLock   sync.Mutex
}

// NewMatchingService creates a new MatchingService
func NewMatchingService(
	clickhouse *chClient.Client,
	sceneRepo data.SceneRepository,
	dupGroupRepo data.DuplicateGroupRepository,
	dupConfigRepo data.DuplicationConfigRepository,
	logger *zap.Logger,
) *MatchingService {
	return &MatchingService{
		clickhouse:    clickhouse,
		sceneRepo:     sceneRepo,
		dupGroupRepo:  dupGroupRepo,
		dupConfigRepo: dupConfigRepo,
		logger:        logger.With(zap.String("component", "matching_service")),
	}
}

// IndexFingerprint adds a scene's fingerprint to the ClickHouse inverted index
func (ms *MatchingService) IndexFingerprint(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch fpType {
	case "audio":
		if len(audioFP) == 0 {
			return nil
		}
		return ms.clickhouse.InsertAudioFingerprints(ctx, sceneID, audioFP)
	case "visual":
		if len(visualFP) == 0 {
			return nil
		}
		return ms.clickhouse.InsertVisualFingerprints(ctx, sceneID, visualFP)
	default:
		return fmt.Errorf("unknown fingerprint type: %s", fpType)
	}
}

// FindMatches looks up a fingerprint against the existing index and returns matches
func (ms *MatchingService) FindMatches(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) ([]matching.MatchResult, error) {
	// Load matching config
	cfg, err := ms.dupConfigRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get duplication config: %w", err)
	}
	// Use defaults if no config record exists
	densityThreshold := 0.50
	audioMinHashes := 80
	audioMaxHashOccurrences := 10
	audioMinSpan := 160
	visualHammingMax := 5
	visualMinFrames := 20
	visualMinSpan := 30
	deltaTolerance := 2

	if cfg != nil {
		densityThreshold = cfg.AudioDensityThreshold
		audioMinHashes = cfg.AudioMinHashes
		audioMaxHashOccurrences = cfg.AudioMaxHashOccurrences
		audioMinSpan = cfg.AudioMinSpan
		visualHammingMax = cfg.VisualHammingMax
		visualMinFrames = cfg.VisualMinFrames
		visualMinSpan = cfg.VisualMinSpan
		deltaTolerance = cfg.DeltaTolerance
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch fpType {
	case "audio":
		return ms.findAudioMatches(ctx, sceneID, audioFP, audioMinHashes, densityThreshold, deltaTolerance, audioMaxHashOccurrences, audioMinSpan)
	case "visual":
		return ms.findVisualMatches(ctx, sceneID, visualFP, visualHammingMax, visualMinFrames, densityThreshold, deltaTolerance, visualMinSpan)
	default:
		return nil, fmt.Errorf("unknown fingerprint type: %s", fpType)
	}
}

func (ms *MatchingService) findAudioMatches(ctx context.Context, sceneID uint, hashes []int32, minHashes int, densityThreshold float64, deltaTolerance int, maxSceneFreq int, minSpan int) ([]matching.MatchResult, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	// Lookup hashes in ClickHouse with server-side popular hash filtering
	chHits, err := ms.clickhouse.LookupAudioHashesFiltered(ctx, hashes, maxSceneFreq)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup audio hashes: %w", err)
	}

	// Convert ClickHouse hits to matching package types
	hits := make(map[int32][]matching.AudioHit, len(chHits))
	for hash, chHitList := range chHits {
		audioHits := make([]matching.AudioHit, len(chHitList))
		for i, h := range chHitList {
			audioHits[i] = matching.AudioHit{SceneID: h.SceneID, Offset: h.Offset}
		}
		hits[hash] = audioHits
	}

	return matching.FindAudioMatches(sceneID, hashes, hits, minHashes, densityThreshold, deltaTolerance, minSpan), nil
}

func (ms *MatchingService) findVisualMatches(ctx context.Context, sceneID uint, hashes []uint64, hammingMax, minFrames int, densityThreshold float64, deltaTolerance int, minSpan int) ([]matching.MatchResult, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	lookupFn := func(chunks []uint16, chunkIdx uint8) ([]matching.VisualHit, error) {
		chHits, err := ms.clickhouse.LookupVisualChunks(ctx, chunks, chunkIdx)
		if err != nil {
			return nil, err
		}
		visualHits := make([]matching.VisualHit, len(chHits))
		for i, h := range chHits {
			visualHits[i] = matching.VisualHit{SceneID: h.SceneID, FrameOffset: h.FrameOffset, FullHash: h.FullHash}
		}
		return visualHits, nil
	}

	return matching.FindVisualMatches(sceneID, hashes, lookupFn, hammingMax, minFrames, densityThreshold, deltaTolerance, minSpan)
}

// ProcessMatches creates or merges duplicate groups based on match results
func (ms *MatchingService) ProcessMatches(sceneID uint, matches []matching.MatchResult) error {
	if len(matches) == 0 {
		return nil
	}

	ms.processLock.Lock()
	defer ms.processLock.Unlock()

	// Collect all matched scene IDs
	matchedSceneIDs := make([]uint, len(matches))
	matchMap := make(map[uint]matching.MatchResult)
	for i, m := range matches {
		matchedSceneIDs[i] = m.SceneID
		matchMap[m.SceneID] = m
	}

	// Check which matched scenes already belong to groups
	existingGroups, err := ms.dupGroupRepo.GetGroupsBySceneIDs(matchedSceneIDs)
	if err != nil {
		return fmt.Errorf("failed to get existing groups: %w", err)
	}

	// Also check if the query scene itself is in a group
	queryGroup, err := ms.dupGroupRepo.GetGroupBySceneID(sceneID)
	if err != nil {
		return fmt.Errorf("failed to get query scene group: %w", err)
	}

	// Collect unique group IDs
	groupIDs := make(map[uint]bool)
	if queryGroup != nil {
		groupIDs[queryGroup.ID] = true
	}
	for _, groupID := range existingGroups {
		groupIDs[groupID] = true
	}

	// Filter out resolved/dismissed groups. When a new duplicate is detected
	// against a scene that was already in a resolved group, we create a fresh
	// unresolved group rather than reopening the old one.
	for gid := range groupIDs {
		group, err := ms.dupGroupRepo.GetByID(gid)
		if err != nil {
			return fmt.Errorf("failed to get group %d status: %w", gid, err)
		}
		if group.Status != "unresolved" {
			delete(groupIDs, gid)
		}
	}

	switch len(groupIDs) {
	case 0:
		// No existing groups - create a new one (wrapped in transaction)
		return ms.dupGroupRepo.RunInTransaction(func(txRepo data.DuplicateGroupRepository) error {
			return ms.createNewGroupTx(txRepo, sceneID, matches)
		})
	case 1:
		// One existing group - add the new scene to it (wrapped in transaction)
		var targetGroupID uint
		for gid := range groupIDs {
			targetGroupID = gid
		}
		return ms.dupGroupRepo.RunInTransaction(func(txRepo data.DuplicateGroupRepository) error {
			return ms.addToGroupTx(txRepo, targetGroupID, sceneID, matchMap)
		})
	default:
		// Multiple groups - merge them all
		var targetGroupID uint
		var sourceGroupIDs []uint
		first := true
		for gid := range groupIDs {
			if first {
				targetGroupID = gid
				first = false
			} else {
				sourceGroupIDs = append(sourceGroupIDs, gid)
			}
		}
		// Merge source groups into target
		if err := ms.dupGroupRepo.MergeGroups(targetGroupID, sourceGroupIDs); err != nil {
			return fmt.Errorf("failed to merge groups: %w", err)
		}
		// Add the new scene if not already a member (wrapped in transaction)
		return ms.dupGroupRepo.RunInTransaction(func(txRepo data.DuplicateGroupRepository) error {
			return ms.addToGroupTx(txRepo, targetGroupID, sceneID, matchMap)
		})
	}
}

func (ms *MatchingService) createNewGroupTx(txRepo data.DuplicateGroupRepository, sceneID uint, matches []matching.MatchResult) error {
	group := &data.DuplicateGroup{
		Status:     "unresolved",
		SceneCount: len(matches) + 1,
	}
	if err := txRepo.Create(group); err != nil {
		return fmt.Errorf("failed to create duplicate group: %w", err)
	}

	// Add the query scene as a member
	if err := txRepo.AddMember(&data.DuplicateGroupMember{
		GroupID:         group.ID,
		SceneID:         sceneID,
		ConfidenceScore: 1.0, // Self-match is 100%
		MatchType:       matches[0].MatchType,
	}); err != nil {
		return fmt.Errorf("failed to add query scene to group: %w", err)
	}

	// Add matched scenes
	for _, m := range matches {
		if err := txRepo.AddMember(&data.DuplicateGroupMember{
			GroupID:         group.ID,
			SceneID:         m.SceneID,
			ConfidenceScore: m.ConfidenceScore,
			MatchType:       m.MatchType,
		}); err != nil {
			return fmt.Errorf("failed to add matched scene %d to group: %w", m.SceneID, err)
		}
	}

	// Auto-score best variant (uses sceneRepo which is read-only, fine outside tx)
	ms.autoScoreBestTx(txRepo, group.ID)

	ms.logger.Info("Created new duplicate group",
		zap.Uint("group_id", group.ID),
		zap.Int("member_count", len(matches)+1),
	)

	return nil
}

func (ms *MatchingService) addToGroupTx(txRepo data.DuplicateGroupRepository, groupID uint, sceneID uint, matchMap map[uint]matching.MatchResult) error {
	// Check if already a member
	existing, err := ms.dupGroupRepo.GetGroupBySceneID(sceneID)
	if err != nil {
		return fmt.Errorf("failed to check existing membership: %w", err)
	}
	if existing != nil && existing.ID == groupID {
		return nil // Already in this group
	}

	// Determine best confidence from matches
	var bestScore float64
	var matchType string
	for _, m := range matchMap {
		if m.ConfidenceScore > bestScore {
			bestScore = m.ConfidenceScore
			matchType = m.MatchType
		}
	}
	if matchType == "" {
		matchType = "audio"
	}

	if err := txRepo.AddMember(&data.DuplicateGroupMember{
		GroupID:         groupID,
		SceneID:         sceneID,
		ConfidenceScore: bestScore,
		MatchType:       matchType,
	}); err != nil {
		return fmt.Errorf("failed to add scene to group: %w", err)
	}

	if err := txRepo.UpdateSceneCount(groupID); err != nil {
		ms.logger.Error("Failed to update scene count", zap.Uint("group_id", groupID), zap.Error(err))
	}

	ms.autoScoreBestTx(txRepo, groupID)

	return nil
}

// autoScoreBestTx uses a transactional repo for group operations but reads scenes from the main repo
func (ms *MatchingService) autoScoreBestTx(txRepo data.DuplicateGroupRepository, groupID uint) {
	group, err := txRepo.GetByIDWithMembers(groupID)
	if err != nil || group == nil {
		return
	}

	var bestSceneID uint
	var bestScore int64

	for _, member := range group.Members {
		scene, err := ms.sceneRepo.GetByID(member.SceneID)
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
		if err := txRepo.SetBestScene(groupID, bestSceneID); err != nil {
			ms.logger.Error("Failed to set best scene", zap.Uint("group_id", groupID), zap.Error(err))
		}
	}
}

// scoreScene computes a quality score for a scene
func scoreScene(scene *data.Scene) int64 {
	score := int64(scene.Duration) * 1000
	score += int64(scene.Width) * int64(scene.Height)

	// Codec ranking
	switch scene.VideoCodec {
	case "av1":
		score += 3_000_000
	case "hevc", "h265":
		score += 2_000_000
	case "h264":
		score += 1_000_000
	}

	score += int64(scene.BitRate) / 1000

	return score
}

// RemoveSceneFromIndex removes all fingerprint data for a scene from ClickHouse
func (ms *MatchingService) RemoveSceneFromIndex(sceneID uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ms.clickhouse.DeleteSceneFingerprints(ctx, sceneID)
}

// BytesToInt32Slice converts a byte slice back to []int32
func BytesToInt32Slice(b []byte) []int32 {
	if len(b) == 0 {
		return nil
	}
	result := make([]int32, len(b)/4)
	for i := range result {
		result[i] = int32(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return result
}

// BytesToUint64Slice converts a byte slice back to []uint64
func BytesToUint64Slice(b []byte) []uint64 {
	if len(b) == 0 {
		return nil
	}
	result := make([]uint64, len(b)/8)
	for i := range result {
		result[i] = binary.LittleEndian.Uint64(b[i*8:])
	}
	return result
}
