package matching

import (
	"math/bits"
)

// VisualHit represents a hit from the visual fingerprint index
type VisualHit struct {
	SceneID     uint
	FrameOffset uint32
	FullHash    uint64
}

// VisualLookupFn is a function that looks up visual chunk candidates from the index
type VisualLookupFn func(chunks []uint16, chunkIdx uint8) ([]VisualHit, error)

// FindVisualMatches finds duplicate visual matches using bit-partition + diagonal alignment.
// querySceneID is excluded from results.
// minSpan requires a minimum aligned duration before accepting a match (0 = disabled).
func FindVisualMatches(
	querySceneID uint,
	queryHashes []uint64,
	lookupFn VisualLookupFn,
	hammingMax int,
	minFrames int,
	densityThreshold float64,
	deltaTolerance int,
	minSpan int,
) ([]MatchResult, error) {
	if len(queryHashes) == 0 || lookupFn == nil {
		return nil, nil
	}

	if deltaTolerance <= 0 {
		deltaTolerance = 2
	}

	// For each chunk index (0-3), collect unique chunk values and their query frame offsets
	type queryInfo struct {
		frameOffset int
		fullHash    uint64
	}

	// Verified hits: delta-based alignment tracking unique query frame positions.
	// Using unique query positions (not raw hit count) prevents score inflation
	// when the same (queryFrame, candidateFrame) pair is verified across multiple
	// chunk partitions (up to 4x overcounting).
	type binData struct {
		queryPositions map[int]struct{}
		minQuery       int
		maxQuery       int
	}
	sceneBins := make(map[uint]map[int]*binData)

	// Process each of the 4 bit-partitions
	for chunkIdx := uint8(0); chunkIdx < 4; chunkIdx++ {
		// Collect unique chunks for this partition
		chunkMap := make(map[uint16][]queryInfo)
		for frameIdx, hash := range queryHashes {
			chunkValue := uint16((hash >> (chunkIdx * 16)) & 0xFFFF)
			chunkMap[chunkValue] = append(chunkMap[chunkValue], queryInfo{
				frameOffset: frameIdx,
				fullHash:    hash,
			})
		}

		// Batch lookup
		chunks := make([]uint16, 0, len(chunkMap))
		for c := range chunkMap {
			chunks = append(chunks, c)
		}

		candidates, err := lookupFn(chunks, chunkIdx)
		if err != nil {
			return nil, err
		}

		// For each candidate, verify Hamming distance
		for _, candidate := range candidates {
			if candidate.SceneID == querySceneID {
				continue
			}

			candidateChunk := uint16((candidate.FullHash >> (chunkIdx * 16)) & 0xFFFF)
			queryInfos, ok := chunkMap[candidateChunk]
			if !ok {
				continue
			}

			for _, qi := range queryInfos {
				if bits.OnesCount64(qi.fullHash^candidate.FullHash) <= hammingMax {
					delta := int(candidate.FrameOffset) - qi.frameOffset
					binKey := floorDiv(delta, deltaTolerance)

					if sceneBins[candidate.SceneID] == nil {
						sceneBins[candidate.SceneID] = make(map[int]*binData)
					}
					bd := sceneBins[candidate.SceneID][binKey]
					if bd == nil {
						bd = &binData{
							queryPositions: make(map[int]struct{}),
							minQuery:       qi.frameOffset,
							maxQuery:       qi.frameOffset,
						}
						sceneBins[candidate.SceneID][binKey] = bd
					}
					bd.queryPositions[qi.frameOffset] = struct{}{}
					if qi.frameOffset < bd.minQuery {
						bd.minQuery = qi.frameOffset
					}
					if qi.frameOffset > bd.maxQuery {
						bd.maxQuery = qi.frameOffset
					}
				}
			}
		}
	}

	// Score and filter results: evaluate all bins against all three gates,
	// then select the one with the highest density score.
	var results []MatchResult
	for sceneID, bins := range sceneBins {
		var bestScore float64
		matched := false

		for _, bd := range bins {
			count := len(bd.queryPositions)
			if count < minFrames {
				continue
			}

			span := bd.maxQuery - bd.minQuery + 1
			if span <= 0 {
				span = 1
			}

			if minSpan > 0 && span < minSpan {
				continue
			}

			score := float64(count) / float64(span)
			if score > 1.0 {
				score = 1.0
			}

			if score >= densityThreshold && score > bestScore {
				bestScore = score
				matched = true
			}
		}

		if matched {
			results = append(results, MatchResult{
				SceneID:         sceneID,
				ConfidenceScore: bestScore,
				MatchType:       "visual",
			})
		}
	}

	return results, nil
}
