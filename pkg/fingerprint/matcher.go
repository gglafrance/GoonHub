package fingerprint

// MatchResult contains the result of comparing two hash sequences.
type MatchResult struct {
	SourceSceneID uint
	TargetSceneID uint
	MatchPercent  float64
	FrameOffset   int
	MatchedFrames int
	TotalFrames   int
}

// FindMatches performs sliding window sequence matching between two hash sequences.
// It slides the shorter sequence along the longer one, counting frames where the
// hamming distance is within threshold. Returns the best match if it exceeds
// the minimum match threshold percentage.
func FindMatches(sourceHashes, targetHashes []uint64, hammingThreshold int, matchThresholdPct float64) *MatchResult {
	if len(sourceHashes) == 0 || len(targetHashes) == 0 {
		return nil
	}

	shorter := sourceHashes
	longer := targetHashes
	swapped := false
	if len(sourceHashes) > len(targetHashes) {
		shorter = targetHashes
		longer = sourceHashes
		swapped = true
	}

	bestMatch := 0
	bestOffset := 0
	shortLen := len(shorter)

	// Slide shorter along longer
	maxOffset := len(longer) - shortLen
	for offset := 0; offset <= maxOffset; offset++ {
		matches := 0
		for i := range shortLen {
			if HammingDistance(shorter[i], longer[offset+i]) <= hammingThreshold {
				matches++
			}
		}
		if matches > bestMatch {
			bestMatch = matches
			bestOffset = offset
		}
	}

	matchPct := float64(bestMatch) / float64(shortLen) * 100.0
	if matchPct < matchThresholdPct {
		return nil
	}

	frameOffset := bestOffset
	if swapped {
		frameOffset = -frameOffset
	}

	return &MatchResult{
		MatchPercent:  matchPct,
		FrameOffset:   frameOffset,
		MatchedFrames: bestMatch,
		TotalFrames:   shortLen,
	}
}
