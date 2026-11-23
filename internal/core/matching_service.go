package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"goonhub/internal/core/matching"
	"goonhub/internal/data"
	chClient "goonhub/internal/infrastructure/clickhouse"
	"sort"
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
	// Normalize matches so repeated/self hits cannot duplicate memberships.
	matchMap := make(map[uint]matching.MatchResult)
	for _, match := range matches {
		if match.SceneID == sceneID {
			continue
		}
		if previous, ok := matchMap[match.SceneID]; !ok || match.ConfidenceScore > previous.ConfidenceScore {
			matchMap[match.SceneID] = match
		}
	}
	if len(matchMap) == 0 {
		return nil
	}
	matchedIDs := make([]uint, 0, len(matchMap))
	for id := range matchMap {
		matchedIDs = append(matchedIDs, id)
	}
	sort.Slice(matchedIDs, func(i, j int) bool { return matchedIDs[i] < matchedIDs[j] })
	matches = make([]matching.MatchResult, 0, len(matchedIDs))
	for _, id := range matchedIDs {
		matches = append(matches, matchMap[id])
	}
	sceneIDs := append([]uint{sceneID}, matchedIDs...)
	// Every segment written below refers to scenes of this set (query and matched scenes)
	durations := ms.sceneDurations(sceneIDs)

	ms.processLock.Lock()
	defer ms.processLock.Unlock()

	return ms.dupGroupRepo.RunInTransaction(func(txRepo data.DuplicateGroupRepository) error {
		memberships, err := txRepo.GetGroupsBySceneIDs(sceneIDs)
		if err != nil {
			return fmt.Errorf("failed to get existing memberships: %w", err)
		}
		groupSet := make(map[uint]bool)
		for _, groupID := range memberships {
			groupSet[groupID] = true
		}
		if len(groupSet) == 0 {
			return ms.createNewGroupTx(txRepo, sceneID, matches, durations)
		}

		groupIDs := make([]uint, 0, len(groupSet))
		for id := range groupSet {
			groupIDs = append(groupIDs, id)
		}
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
		targetID := groupIDs[0]
		target, err := txRepo.GetByID(targetID)
		if err != nil {
			return fmt.Errorf("failed to get group %d: %w", targetID, err)
		}

		// Reprocessing known members must not undo a resolution or dismissal.
		// New members reuse and reopen the group to preserve unique memberships.
		if len(groupIDs) == 1 && target.Status != "unresolved" && len(memberships) == len(sceneIDs) {
			return nil
		}
		if len(groupIDs) > 1 {
			if err := txRepo.MergeGroups(targetID, groupIDs[1:]); err != nil {
				return fmt.Errorf("failed to merge groups: %w", err)
			}
		}
		if err := ms.addToGroupTx(txRepo, targetID, sceneID, matchMap, durations); err != nil {
			return err
		}
		if target.Status != "unresolved" {
			if err := txRepo.UpdateStatus(targetID, "unresolved", nil); err != nil {
				return fmt.Errorf("failed to reopen group %d: %w", targetID, err)
			}
		}
		return nil
	})
}

// sceneDurations returns the durations (seconds) of the given scenes, used to classify the
// relation of group members. On failure the relations stay unknown (and so not safe for
// automatic resolution) until the next startup backfill.
func (ms *MatchingService) sceneDurations(sceneIDs []uint) map[uint]int {
	durations := make(map[uint]int, len(sceneIDs))
	scenes, err := ms.sceneRepo.GetByIDsIncludingTrashed(sceneIDs)
	if err != nil {
		ms.logger.Warn("Failed to load scene durations, duplicate relations left unknown", zap.Error(err))
		return durations
	}
	for _, scene := range scenes {
		durations[scene.ID] = scene.Duration
	}
	return durations
}

func (ms *MatchingService) createNewGroupTx(txRepo data.DuplicateGroupRepository, sceneID uint, matches []matching.MatchResult, durations map[uint]int) error {
	group := &data.DuplicateGroup{
		Status:     "unresolved",
		SceneCount: len(matches) + 1,
	}
	if err := txRepo.Create(group); err != nil {
		return fmt.Errorf("failed to create duplicate group: %w", err)
	}

	// Add the query scene as a member (its segment is the one of its best match)
	queryMember := &data.DuplicateGroupMember{
		GroupID:         group.ID,
		SceneID:         sceneID,
		ConfidenceScore: 1.0, // Self-match is 100%
		MatchType:       matches[0].MatchType,
	}
	setMemberSegment(queryMember, bestMatchOf(matches), sceneID, true)
	queryMember.Relation = relationOfMember(queryMember, durations)
	if err := txRepo.AddMember(queryMember); err != nil {
		return fmt.Errorf("failed to add query scene to group: %w", err)
	}

	// Add matched scenes
	for _, m := range matches {
		member := &data.DuplicateGroupMember{
			GroupID:         group.ID,
			SceneID:         m.SceneID,
			ConfidenceScore: m.ConfidenceScore,
			MatchType:       m.MatchType,
		}
		setMemberSegment(member, m, sceneID, false)
		member.Relation = relationOfMember(member, durations)
		if err := txRepo.AddMember(member); err != nil {
			return fmt.Errorf("failed to add matched scene %d to group: %w", m.SceneID, err)
		}
	}

	// Auto-score best variant (uses sceneRepo which is read-only, fine outside tx)
	if err := ms.autoScoreBestTx(txRepo, group.ID); err != nil {
		return err
	}

	ms.logger.Info("Created new duplicate group",
		zap.Uint("group_id", group.ID),
		zap.Int("member_count", len(matches)+1),
	)

	return nil
}

func (ms *MatchingService) addToGroupTx(txRepo data.DuplicateGroupRepository, groupID uint, sceneID uint, matchMap map[uint]matching.MatchResult, durations map[uint]int) error {
	sceneIDs := make([]uint, 0, len(matchMap)+1)
	sceneIDs = append(sceneIDs, sceneID)
	for id := range matchMap {
		sceneIDs = append(sceneIDs, id)
	}
	sort.Slice(sceneIDs, func(i, j int) bool { return sceneIDs[i] < sceneIDs[j] })
	memberships, err := txRepo.GetGroupsBySceneIDs(sceneIDs)
	if err != nil {
		return fmt.Errorf("failed to check existing memberships: %w", err)
	}

	var bestMatch matching.MatchResult
	for _, match := range matchMap {
		if match.ConfidenceScore > bestMatch.ConfidenceScore {
			bestMatch = match
		}
	}
	if bestMatch.MatchType == "" {
		bestMatch.MatchType = "audio"
	}
	for _, id := range sceneIDs {
		if existingID, ok := memberships[id]; ok {
			if existingID != groupID {
				return fmt.Errorf("scene %d still belongs to group %d", id, existingID)
			}
			// Known member: record its matching segment if it has none yet (older groups)
			match := matchMap[id]
			if id == sceneID {
				match = bestMatch
			}
			segment := &data.DuplicateGroupMember{GroupID: groupID, SceneID: id}
			setMemberSegment(segment, match, sceneID, id == sceneID)
			segment.Relation = relationOfMember(segment, durations)
			if segment.MatchStart != nil {
				if err := txRepo.FillMemberSegment(segment); err != nil {
					return fmt.Errorf("failed to record matching segment of scene %d: %w", id, err)
				}
			}
			continue
		}
		match := matchMap[id]
		if id == sceneID {
			match = bestMatch
		}
		member := &data.DuplicateGroupMember{
			GroupID: groupID, SceneID: id,
			ConfidenceScore: match.ConfidenceScore, MatchType: match.MatchType,
		}
		setMemberSegment(member, match, sceneID, id == sceneID)
		member.Relation = relationOfMember(member, durations)
		if err := txRepo.AddMember(member); err != nil {
			return fmt.Errorf("failed to add scene %d to group: %w", id, err)
		}
	}
	if err := txRepo.UpdateSceneCount(groupID); err != nil {
		return fmt.Errorf("failed to update scene count: %w", err)
	}
	return ms.autoScoreBestTx(txRepo, groupID)
}

func (ms *MatchingService) autoScoreBestTx(txRepo data.DuplicateGroupRepository, groupID uint) error {
	group, err := txRepo.GetByIDWithMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get group members for scoring: %w", err)
	}

	criteria := DefaultBestCriteria
	if ms.dupConfigRepo != nil {
		if cfg, err := ms.dupConfigRepo.Get(); err == nil {
			criteria = bestCriteriaFromConfig(cfg)
		}
	}
	var best *data.Scene
	for _, member := range group.Members {
		scene, err := ms.sceneRepo.GetByIDIncludingTrashed(member.SceneID)
		if err != nil {
			return fmt.Errorf("failed to get scene %d for scoring: %w", member.SceneID, err)
		}
		if scene.TrashedAt != nil {
			continue
		}
		if best == nil || betterSceneBy(scene, best, criteria) {
			best = scene
		}
	}
	var bestSceneID uint
	if best != nil {
		bestSceneID = best.ID
	}
	if bestSceneID > 0 {
		if err := txRepo.SetBestScene(groupID, bestSceneID); err != nil {
			return fmt.Errorf("failed to set best scene: %w", err)
		}
	}
	return nil
}

// segmentBackfillBatch is how many members BackfillMemberSegments reads per query.
const segmentBackfillBatch = 200

// BackfillMemberSegments computes the matching segment (and relation) of the group members
// that have none: groups recorded before segments existed, and the segments a migration
// cleared because a bug stored "0 to one fingerprint step" for every member (the job result
// dropped the aligned positions). Each member is matched again from its stored fingerprints
// against the index, and the best match with another member of its group gives the
// segment. No video is decoded again. Returns how many members got a segment.
func (ms *MatchingService) BackfillMemberSegments() (int, error) {
	filled := 0
	groupMembers := map[uint]map[uint]bool{}
	var afterID uint
	for {
		rows, err := ms.dupGroupRepo.ListMembersWithoutSegment(afterID, segmentBackfillBatch)
		if err != nil {
			return filled, fmt.Errorf("failed to list members without segment: %w", err)
		}
		for i := range rows {
			member := &rows[i]
			afterID = member.ID
			others, ok := groupMembers[member.GroupID]
			if !ok {
				group, err := ms.dupGroupRepo.GetByIDWithMembers(member.GroupID)
				if err != nil {
					ms.logger.Warn("Segment backfill: failed to load group", zap.Uint("group_id", member.GroupID), zap.Error(err))
					continue
				}
				others = make(map[uint]bool, len(group.Members))
				for _, m := range group.Members {
					others[m.SceneID] = true
				}
				groupMembers[member.GroupID] = others
			}
			best, found, err := ms.bestGroupMatch(member, others)
			if err != nil {
				ms.logger.Warn("Segment backfill: matching failed", zap.Uint("scene_id", member.SceneID), zap.Error(err))
				continue
			}
			if !found {
				continue
			}
			segment := &data.DuplicateGroupMember{GroupID: member.GroupID, SceneID: member.SceneID}
			setMemberSegment(segment, best, member.SceneID, true)
			segment.Relation = relationOfMember(segment, ms.sceneDurations([]uint{member.SceneID, best.SceneID}))
			if err := ms.dupGroupRepo.FillMemberSegment(segment); err != nil {
				return filled, fmt.Errorf("failed to store the segment of scene %d: %w", member.SceneID, err)
			}
			filled++
		}
		if len(rows) < segmentBackfillBatch {
			return filled, nil
		}
	}
}

// bestGroupMatch matches a member's stored fingerprints (its match type first) and returns
// the best match with another scene of its group.
func (ms *MatchingService) bestGroupMatch(member *data.DuplicateGroupMember, groupScenes map[uint]bool) (matching.MatchResult, bool, error) {
	scenes, err := ms.sceneRepo.GetByIDsIncludingTrashed([]uint{member.SceneID})
	if err != nil {
		return matching.MatchResult{}, false, fmt.Errorf("failed to load scene: %w", err)
	}
	if len(scenes) == 0 {
		return matching.MatchResult{}, false, nil
	}
	scene := scenes[0]
	types := []string{"audio", "visual"}
	if member.MatchType == "visual" {
		types = []string{"visual", "audio"}
	}
	for _, fpType := range types {
		var matches []matching.MatchResult
		switch {
		case fpType == "audio" && len(scene.AudioFingerprint) > 0:
			matches, err = ms.FindMatches(scene.ID, "audio", BytesToInt32Slice(scene.AudioFingerprint), nil)
		case fpType == "visual" && len(scene.VisualFingerprint) > 0:
			matches, err = ms.FindMatches(scene.ID, "visual", nil, BytesToUint64Slice(scene.VisualFingerprint))
		default:
			continue
		}
		if err != nil {
			return matching.MatchResult{}, false, err
		}
		var best matching.MatchResult
		for _, m := range matches {
			if m.SceneID != scene.ID && groupScenes[m.SceneID] && m.ConfidenceScore > best.ConfidenceScore {
				best = m
			}
		}
		if best.SceneID != 0 {
			return best, true, nil
		}
	}
	return matching.MatchResult{}, false, nil
}

// StartSegmentBackfill runs BackfillMemberSegments once in the background.
func (ms *MatchingService) StartSegmentBackfill() {
	go func() {
		filled, err := ms.BackfillMemberSegments()
		if err != nil {
			ms.logger.Warn("Duplicate segment backfill failed", zap.Int("filled", filled), zap.Error(err))
			return
		}
		if filled > 0 {
			ms.logger.Info("Recomputed duplicate matching segments", zap.Int("members", filled))
		}
	}()
}

// Fingerprint position durations, used to convert matched segments to seconds.
const (
	// chromaprint raw fingerprint: one item every 4096/3 samples at 11025 Hz
	audioFingerprintStepSeconds = 4096.0 / 3.0 / 11025.0
	// visual dHash: one frame every 2 seconds (fps=1/2, see pkg/dhash)
	visualFingerprintStepSeconds = 2.0
)

// bestMatchOf returns the match with the highest confidence.
func bestMatchOf(matches []matching.MatchResult) matching.MatchResult {
	var best matching.MatchResult
	for _, m := range matches {
		if m.ConfidenceScore > best.ConfidenceScore {
			best = m
		}
	}
	return best
}

// setMemberSegment stores on a group member its matching segment (seconds) and the
// corresponding segment of the other scene. forQuery is true for the query scene itself:
// its segment is the query side of the match, and the reference is the matched scene.
func setMemberSegment(member *data.DuplicateGroupMember, m matching.MatchResult, querySceneID uint, forQuery bool) {
	if m.SceneID == 0 || m.QueryEnd < m.QueryStart {
		return
	}
	step := audioFingerprintStepSeconds
	if m.MatchType == "visual" {
		step = visualFingerprintStepSeconds
	}
	queryStart := float64(m.QueryStart) * step
	queryEnd := float64(m.QueryEnd+1) * step
	otherStart := max(0, float64(m.QueryStart+m.Delta)*step)
	otherEnd := max(0, float64(m.QueryEnd+1+m.Delta)*step)

	refID := querySceneID
	if forQuery {
		refID = m.SceneID
		member.MatchStart, member.MatchEnd = &queryStart, &queryEnd
		member.RefStart, member.RefEnd = &otherStart, &otherEnd
	} else {
		member.MatchStart, member.MatchEnd = &otherStart, &otherEnd
		member.RefStart, member.RefEnd = &queryStart, &queryEnd
	}
	member.RefSceneID = &refID
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
