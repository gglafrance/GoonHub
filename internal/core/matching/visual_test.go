package matching

import (
	"fmt"
	"math/bits"
	"testing"
)

// makeLookupFn creates a VisualLookupFn that returns predetermined hits.
// The hitsByChunkIdx map is keyed by chunkIdx, then by chunk value.
// For each incoming chunk value at the given chunkIdx, the corresponding
// VisualHit slice is returned.
func makeLookupFn(hitsByChunkIdx map[uint8]map[uint16][]VisualHit) VisualLookupFn {
	return func(chunks []uint16, chunkIdx uint8) ([]VisualHit, error) {
		chunkHits, ok := hitsByChunkIdx[chunkIdx]
		if !ok {
			return nil, nil
		}
		var result []VisualHit
		for _, c := range chunks {
			if hits, found := chunkHits[c]; found {
				result = append(result, hits...)
			}
		}
		return result, nil
	}
}

// buildHash constructs a 64-bit hash from four 16-bit chunks.
// chunk0 occupies bits 0-15, chunk1 bits 16-31, chunk2 bits 32-47, chunk3 bits 48-63.
func buildHash(chunk0, chunk1, chunk2, chunk3 uint16) uint64 {
	return uint64(chunk0) |
		(uint64(chunk1) << 16) |
		(uint64(chunk2) << 32) |
		(uint64(chunk3) << 48)
}

// flipBits flips exactly n bits in the given hash, starting from the lowest bit positions.
func flipBits(hash uint64, n int) uint64 {
	for i := 0; i < n; i++ {
		hash ^= 1 << i
	}
	return hash
}

func TestFindVisualMatches_EmptyInputs(t *testing.T) {
	tests := []struct {
		name        string
		queryHashes []uint64
		lookupFn    VisualLookupFn
	}{
		{
			name:        "nil hashes",
			queryHashes: nil,
			lookupFn:    func(chunks []uint16, chunkIdx uint8) ([]VisualHit, error) { return nil, nil },
		},
		{
			name:        "empty hashes",
			queryHashes: []uint64{},
			lookupFn:    func(chunks []uint16, chunkIdx uint8) ([]VisualHit, error) { return nil, nil },
		},
		{
			name:        "nil lookupFn",
			queryHashes: []uint64{0xDEADBEEF},
			lookupFn:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := FindVisualMatches(1, tt.queryHashes, tt.lookupFn, 5, 1, 0.5, 2, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if results != nil {
				t.Fatalf("expected nil results, got %v", results)
			}
		})
	}
}

func TestFindVisualMatches_NoSelfMatch(t *testing.T) {
	querySceneID := uint(42)
	hash := buildHash(0x1111, 0x2222, 0x3333, 0x4444)
	queryHashes := []uint64{hash, hash, hash, hash, hash}

	// Return candidates that all belong to the query scene itself
	lookup := hitsByChunkIdxForScene(querySceneID, queryHashes, 0)

	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 5, 1, 0.5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results (self-match excluded), got %d", len(results))
	}
}

// hitsByChunkIdxForScene builds a lookup map where a given scene has hits for all
// query hashes at matching frame offsets, with an optional constant frame offset shift.
func hitsByChunkIdxForScene(sceneID uint, hashes []uint64, offsetShift int) map[uint8]map[uint16][]VisualHit {
	m := make(map[uint8]map[uint16][]VisualHit)
	for chunkIdx := uint8(0); chunkIdx < 4; chunkIdx++ {
		chunkMap := make(map[uint16][]VisualHit)
		for frameIdx, hash := range hashes {
			chunkValue := uint16((hash >> (chunkIdx * 16)) & 0xFFFF)
			chunkMap[chunkValue] = append(chunkMap[chunkValue], VisualHit{
				SceneID:     sceneID,
				FrameOffset: uint32(frameIdx + offsetShift),
				FullHash:    hash,
			})
		}
		m[chunkIdx] = chunkMap
	}
	return m
}

// mergeHitMaps merges multiple hit maps into one.
func mergeHitMaps(maps ...map[uint8]map[uint16][]VisualHit) map[uint8]map[uint16][]VisualHit {
	result := make(map[uint8]map[uint16][]VisualHit)
	for _, m := range maps {
		for chunkIdx, chunkMap := range m {
			if result[chunkIdx] == nil {
				result[chunkIdx] = make(map[uint16][]VisualHit)
			}
			for cv, hits := range chunkMap {
				result[chunkIdx][cv] = append(result[chunkIdx][cv], hits...)
			}
		}
	}
	return result
}

func TestFindVisualMatches_ExactMatch(t *testing.T) {
	querySceneID := uint(1)
	candidateSceneID := uint(2)
	queryHashes := make([]uint64, 10)
	for i := range queryHashes {
		queryHashes[i] = buildHash(uint16(i+1), uint16(i+100), uint16(i+200), uint16(i+300))
	}

	// Scene 2 has identical hashes at the same frame offsets
	lookup := hitsByChunkIdxForScene(candidateSceneID, queryHashes, 0)

	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SceneID != candidateSceneID {
		t.Fatalf("expected scene ID %d, got %d", candidateSceneID, results[0].SceneID)
	}
	if results[0].MatchType != "visual" {
		t.Fatalf("expected match type 'visual', got %q", results[0].MatchType)
	}
	// 10 exact matches in a span of 10. With unique query position deduplication
	// across chunk partitions, score should be exactly 1.0.
	if results[0].ConfidenceScore < 0.99 || results[0].ConfidenceScore > 1.01 {
		t.Fatalf("expected confidence score ~1.0, got %f", results[0].ConfidenceScore)
	}
}

func TestFindVisualMatches_HammingWithinMax(t *testing.T) {
	querySceneID := uint(1)
	candidateSceneID := uint(2)

	// Create 10 query hashes
	queryHashes := make([]uint64, 10)
	for i := range queryHashes {
		queryHashes[i] = buildHash(uint16(i+1), uint16(i+100), uint16(i+200), uint16(i+300))
	}

	hammingMax := 5

	// Create candidate hashes that differ by exactly 3 bits from query hashes.
	// We flip 3 bits in the upper chunk (bits 48-63) so that the lower chunks
	// still match for lookup, but full Hamming distance is 3.
	lookup := make(map[uint8]map[uint16][]VisualHit)
	for chunkIdx := uint8(0); chunkIdx < 4; chunkIdx++ {
		chunkMap := make(map[uint16][]VisualHit)
		for frameIdx, hash := range queryHashes {
			chunkValue := uint16((hash >> (chunkIdx * 16)) & 0xFFFF)
			// Flip 3 bits in the hash (in bits 48-50, affecting chunk 3)
			candidateHash := hash ^ (0x7 << 48)
			dist := bits.OnesCount64(hash ^ candidateHash)
			if dist != 3 {
				t.Fatalf("expected hamming distance 3, got %d", dist)
			}
			chunkMap[chunkValue] = append(chunkMap[chunkValue], VisualHit{
				SceneID:     candidateSceneID,
				FrameOffset: uint32(frameIdx),
				FullHash:    candidateHash,
			})
		}
		lookup[chunkIdx] = chunkMap
	}

	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), hammingMax, 5, 0.5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SceneID != candidateSceneID {
		t.Fatalf("expected scene ID %d, got %d", candidateSceneID, results[0].SceneID)
	}
}

func TestFindVisualMatches_HammingExceedsMax(t *testing.T) {
	querySceneID := uint(1)
	candidateSceneID := uint(2)
	hammingMax := 5

	// Create 10 query hashes
	queryHashes := make([]uint64, 10)
	for i := range queryHashes {
		queryHashes[i] = buildHash(uint16(i+1), uint16(i+100), uint16(i+200), uint16(i+300))
	}

	// Create candidate hashes that differ by 10 bits. We flip 10 bits in bits 32-41
	// (affecting chunk 2, bits 32-47). We register them under chunks 0 and 1 so
	// the lookup finds them, but full hamming verification should reject them.
	lookup := make(map[uint8]map[uint16][]VisualHit)
	// Only register under chunk indices 0 and 1 (whose bits are unchanged)
	for chunkIdx := uint8(0); chunkIdx < 2; chunkIdx++ {
		chunkMap := make(map[uint16][]VisualHit)
		for frameIdx, hash := range queryHashes {
			chunkValue := uint16((hash >> (chunkIdx * 16)) & 0xFFFF)
			// Flip 10 bits: bits 32-41
			candidateHash := hash ^ (0x3FF << 32)
			dist := bits.OnesCount64(hash ^ candidateHash)
			if dist != 10 {
				t.Fatalf("expected hamming distance 10, got %d", dist)
			}
			chunkMap[chunkValue] = append(chunkMap[chunkValue], VisualHit{
				SceneID:     candidateSceneID,
				FrameOffset: uint32(frameIdx),
				FullHash:    candidateHash,
			})
		}
		lookup[chunkIdx] = chunkMap
	}

	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), hammingMax, 1, 0.1, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results (hamming distance exceeds max), got %d", len(results))
	}
}

func TestFindVisualMatches_BelowMinFrames(t *testing.T) {
	querySceneID := uint(1)
	candidateSceneID := uint(2)

	// Create 10 query hashes
	queryHashes := make([]uint64, 10)
	for i := range queryHashes {
		queryHashes[i] = buildHash(uint16(i+1), uint16(i+100), uint16(i+200), uint16(i+300))
	}

	// Only provide candidate hits for 3 frames (indices 0, 1, 2)
	lookup := make(map[uint8]map[uint16][]VisualHit)
	for chunkIdx := uint8(0); chunkIdx < 4; chunkIdx++ {
		chunkMap := make(map[uint16][]VisualHit)
		for frameIdx := 0; frameIdx < 3; frameIdx++ {
			hash := queryHashes[frameIdx]
			chunkValue := uint16((hash >> (chunkIdx * 16)) & 0xFFFF)
			chunkMap[chunkValue] = append(chunkMap[chunkValue], VisualHit{
				SceneID:     candidateSceneID,
				FrameOffset: uint32(frameIdx),
				FullHash:    hash,
			})
		}
		lookup[chunkIdx] = chunkMap
	}

	// With unique query position deduplication, 3 frames = 3 unique positions
	// regardless of how many chunk partitions match. minFrames=15 ensures we
	// fall below the threshold.
	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 15, 0.1, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results (below min frames), got %d", len(results))
	}
}

func TestFindVisualMatches_LookupError(t *testing.T) {
	querySceneID := uint(1)
	queryHashes := []uint64{buildHash(0x1111, 0x2222, 0x3333, 0x4444)}
	expectedErr := fmt.Errorf("index lookup failed")

	errorLookup := func(chunks []uint16, chunkIdx uint8) ([]VisualHit, error) {
		return nil, expectedErr
	}

	results, err := FindVisualMatches(querySceneID, queryHashes, errorLookup, 5, 1, 0.5, 2, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %q, got %q", expectedErr.Error(), err.Error())
	}
	if results != nil {
		t.Fatalf("expected nil results on error, got %v", results)
	}
}

func TestFindVisualMatches_OffsetMatch(t *testing.T) {
	querySceneID := uint(1)
	candidateSceneID := uint(2)

	// 20 query hashes with distinct values
	queryHashes := make([]uint64, 20)
	for i := range queryHashes {
		queryHashes[i] = buildHash(
			uint16(i*4+1),
			uint16(i*4+2),
			uint16(i*4+3),
			uint16(i*4+4),
		)
	}

	// Scene 2 has the same hashes but shifted by a constant offset of 100 frames
	offsetShift := 100
	lookup := hitsByChunkIdxForScene(candidateSceneID, queryHashes, offsetShift)

	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SceneID != candidateSceneID {
		t.Fatalf("expected scene ID %d, got %d", candidateSceneID, results[0].SceneID)
	}
	if results[0].MatchType != "visual" {
		t.Fatalf("expected match type 'visual', got %q", results[0].MatchType)
	}
}

func TestFindVisualMatches_ScoreCappedAt1(t *testing.T) {
	// Verify that exact matches across all 4 chunk partitions produce
	// a score of exactly 1.0, not 4.0 (the old buggy behavior).
	querySceneID := uint(1)
	candidateSceneID := uint(2)
	queryHashes := make([]uint64, 20)
	for i := range queryHashes {
		queryHashes[i] = buildHash(uint16(i+1), uint16(i+100), uint16(i+200), uint16(i+300))
	}

	lookup := hitsByChunkIdxForScene(candidateSceneID, queryHashes, 0)

	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ConfidenceScore > 1.01 {
		t.Fatalf("confidence score must not exceed 1.0, got %f", results[0].ConfidenceScore)
	}
	if results[0].ConfidenceScore < 0.99 {
		t.Fatalf("expected confidence score ~1.0 for exact match, got %f", results[0].ConfidenceScore)
	}
}

func TestFindVisualMatches_ChunkPartitioning(t *testing.T) {
	// Verify that chunk extraction correctly maps 16-bit segments of a 64-bit hash.
	// We set up a hash with known chunk values and provide candidates that match
	// only via specific chunk indices.
	querySceneID := uint(1)

	tests := []struct {
		name     string
		chunk0   uint16
		chunk1   uint16
		chunk2   uint16
		chunk3   uint16
		chunkIdx uint8
	}{
		{name: "chunk0_bits_0_15", chunk0: 0xAAAA, chunk1: 0x0000, chunk2: 0x0000, chunk3: 0x0000, chunkIdx: 0},
		{name: "chunk1_bits_16_31", chunk0: 0x0000, chunk1: 0xBBBB, chunk2: 0x0000, chunk3: 0x0000, chunkIdx: 1},
		{name: "chunk2_bits_32_47", chunk0: 0x0000, chunk1: 0x0000, chunk2: 0xCCCC, chunk3: 0x0000, chunkIdx: 2},
		{name: "chunk3_bits_48_63", chunk0: 0x0000, chunk1: 0x0000, chunk2: 0x0000, chunk3: 0xDDDD, chunkIdx: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := buildHash(tt.chunk0, tt.chunk1, tt.chunk2, tt.chunk3)

			// Verify chunk extraction matches what the algorithm does
			extracted := uint16((hash >> (tt.chunkIdx * 16)) & 0xFFFF)
			expectedChunks := [4]uint16{tt.chunk0, tt.chunk1, tt.chunk2, tt.chunk3}
			if extracted != expectedChunks[tt.chunkIdx] {
				t.Fatalf("chunk %d: expected 0x%04X, got 0x%04X",
					tt.chunkIdx, expectedChunks[tt.chunkIdx], extracted)
			}

			// Create enough query hashes for minFrames
			numFrames := 10
			queryHashes := make([]uint64, numFrames)
			for i := range queryHashes {
				queryHashes[i] = hash
			}

			candidateSceneID := uint(100 + tt.chunkIdx)

			// Only provide hits for the specific chunk index being tested.
			// This verifies the algorithm actually queries that chunk index.
			lookup := make(map[uint8]map[uint16][]VisualHit)
			chunkMap := make(map[uint16][]VisualHit)
			for frameIdx := 0; frameIdx < numFrames; frameIdx++ {
				chunkMap[expectedChunks[tt.chunkIdx]] = append(
					chunkMap[expectedChunks[tt.chunkIdx]],
					VisualHit{
						SceneID:     candidateSceneID,
						FrameOffset: uint32(frameIdx),
						FullHash:    hash,
					},
				)
			}
			lookup[tt.chunkIdx] = chunkMap

			results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Should find a match via the single chunk index
			found := false
			for _, r := range results {
				if r.SceneID == candidateSceneID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected match via chunk index %d, got results: %v", tt.chunkIdx, results)
			}
		})
	}
}

func TestFindVisualMatches_MinSpan(t *testing.T) {
	querySceneID := uint(1)
	candidateSceneID := uint(2)

	// Create 10 query hashes with distinct values (span = 10 frames)
	queryHashes := make([]uint64, 10)
	for i := range queryHashes {
		queryHashes[i] = buildHash(uint16(i+1), uint16(i+100), uint16(i+200), uint16(i+300))
	}

	lookup := hitsByChunkIdxForScene(candidateSceneID, queryHashes, 0)

	// Without minSpan: 10 frames in span 10, density=1.0, passes minFrames=5
	results, err := FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result without minSpan, got %d", len(results))
	}

	// With minSpan=30 (~60 seconds at 1 frame/2sec): span of 10 < 30, should be rejected
	results, err = FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results with minSpan=30 (span=10), got %d", len(results))
	}

	// With minSpan=8: span of 10 >= 8, should pass
	results, err = FindVisualMatches(querySceneID, queryHashes, makeLookupFn(lookup), 0, 5, 0.5, 2, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with minSpan=8 (span=10), got %d", len(results))
	}
}
