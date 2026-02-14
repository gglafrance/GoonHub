package matching

// MatchResult represents a match between two scenes
type MatchResult struct {
	SceneID         uint
	ConfidenceScore float64
	MatchType       string // "audio" or "visual"
}

// AudioHit represents a hit from the audio fingerprint index
type AudioHit struct {
	SceneID uint
	Offset  uint32
}

// floorDiv performs floor division (rounds toward negative infinity),
// unlike Go's built-in integer division which truncates toward zero.
// This ensures uniform bin widths: without it, bin 0 covers 3 delta values
// ({-1,0,1}) while other bins cover only 2.
func floorDiv(a, b int) int {
	q := a / b
	// If signs differ and there's a remainder, subtract 1
	if (a^b) < 0 && q*b != a {
		q--
	}
	return q
}

// FindAudioMatches finds duplicate audio matches using diagonal/Hough Transform alignment.
// querySceneID is excluded from results.
// hits is the lookup result from ClickHouse: map[subHash][]AudioHit.
// Popular hash filtering is handled server-side in ClickHouse before hits arrive here.
// minSpan requires a minimum aligned duration before accepting a match (0 = disabled).
// Returns matches that pass both minHashes and densityThreshold gates.
func FindAudioMatches(
	querySceneID uint,
	queryHashes []int32,
	hits map[int32][]AudioHit,
	minHashes int,
	densityThreshold float64,
	deltaTolerance int,
	minSpan int,
) []MatchResult {
	if len(queryHashes) == 0 || len(hits) == 0 {
		return nil
	}

	if deltaTolerance <= 0 {
		deltaTolerance = 2
	}

	// sceneBins maps sceneID -> binKey -> bin tracking unique query positions.
	// Using unique query positions (not raw hit count) prevents score inflation
	// when a single query position produces multiple hits from the same scene
	// (e.g. repeated sub-fingerprint values at different offsets).
	type binData struct {
		queryPositions map[int]struct{}
		minQuery       int
		maxQuery       int
	}
	sceneBins := make(map[uint]map[int]*binData)

	for queryOffset, hash := range queryHashes {
		hashHits, ok := hits[hash]
		if !ok {
			continue
		}
		for _, hit := range hashHits {
			if hit.SceneID == querySceneID {
				continue
			}
			delta := int(hit.Offset) - queryOffset
			binKey := floorDiv(delta, deltaTolerance)

			if sceneBins[hit.SceneID] == nil {
				sceneBins[hit.SceneID] = make(map[int]*binData)
			}
			bd := sceneBins[hit.SceneID][binKey]
			if bd == nil {
				bd = &binData{
					queryPositions: make(map[int]struct{}),
					minQuery:       queryOffset,
					maxQuery:       queryOffset,
				}
				sceneBins[hit.SceneID][binKey] = bd
			}
			bd.queryPositions[queryOffset] = struct{}{}
			if queryOffset < bd.minQuery {
				bd.minQuery = queryOffset
			}
			if queryOffset > bd.maxQuery {
				bd.maxQuery = queryOffset
			}
		}
	}

	var results []MatchResult

	for sceneID, bins := range sceneBins {
		// Evaluate all bins against all three gates, then select the one
		// with the highest density score. This prevents a large sparse bin
		// (high count, low density) from being selected over a smaller dense
		// bin that would have been a valid match.
		var bestScore float64
		matched := false

		for _, bd := range bins {
			count := len(bd.queryPositions)
			if count < minHashes {
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
				MatchType:       "audio",
			})
		}
	}

	return results
}
