package matching

import (
	"testing"
)

func TestFindAudioMatches_EmptyInputs(t *testing.T) {
	tests := []struct {
		name        string
		queryHashes []int32
		hits        map[int32][]AudioHit
	}{
		{
			name:        "nil hashes and nil hits",
			queryHashes: nil,
			hits:        nil,
		},
		{
			name:        "empty hashes slice",
			queryHashes: []int32{},
			hits:        map[int32][]AudioHit{100: {{SceneID: 2, Offset: 0}}},
		},
		{
			name:        "nil hashes with populated hits",
			queryHashes: nil,
			hits:        map[int32][]AudioHit{100: {{SceneID: 2, Offset: 0}}},
		},
		{
			name:        "populated hashes with nil hits",
			queryHashes: []int32{100, 200, 300},
			hits:        nil,
		},
		{
			name:        "populated hashes with empty hits map",
			queryHashes: []int32{100, 200, 300},
			hits:        map[int32][]AudioHit{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := FindAudioMatches(1, tt.queryHashes, tt.hits, 3, 0.5, 2, 0)
			if results != nil {
				t.Fatalf("expected nil results, got %v", results)
			}
		})
	}
}

func TestFindAudioMatches_NoSelfMatch(t *testing.T) {
	querySceneID := uint(1)
	queryHashes := []int32{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}

	// All hits point back to the query scene itself
	hits := make(map[int32][]AudioHit)
	for i, hash := range queryHashes {
		hits[hash] = []AudioHit{{SceneID: querySceneID, Offset: uint32(i)}}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 3, 0.5, 2, 0)
	if results != nil {
		t.Fatalf("expected nil results when all hits are self-matches, got %v", results)
	}
}

func TestFindAudioMatches_PerfectMatch(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)
	numHashes := 20
	queryHashes := make([]int32, numHashes)
	hits := make(map[int32][]AudioHit)

	// Scene B has identical hashes at the same offsets (delta=0 for all)
	for i := 0; i < numHashes; i++ {
		hash := int32(1000 + i)
		queryHashes[i] = hash
		hits[hash] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(i)}}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, 2, 0)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SceneID != matchSceneID {
		t.Errorf("expected SceneID %d, got %d", matchSceneID, results[0].SceneID)
	}
	if results[0].MatchType != "audio" {
		t.Errorf("expected MatchType 'audio', got %q", results[0].MatchType)
	}
	// All 20 hashes in a span of 20, so score = 20/20 = 1.0
	if results[0].ConfidenceScore < 0.99 {
		t.Errorf("expected confidence score ~1.0 for perfect match, got %f", results[0].ConfidenceScore)
	}
}

func TestFindAudioMatches_OffsetMatch(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)
	numHashes := 20
	constantOffset := uint32(100) // Scene B is shifted by 100 positions
	queryHashes := make([]int32, numHashes)
	hits := make(map[int32][]AudioHit)

	for i := 0; i < numHashes; i++ {
		hash := int32(2000 + i)
		queryHashes[i] = hash
		hits[hash] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(i) + constantOffset}}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, 2, 0)

	if len(results) != 1 {
		t.Fatalf("expected 1 result for offset match, got %d", len(results))
	}
	if results[0].SceneID != matchSceneID {
		t.Errorf("expected SceneID %d, got %d", matchSceneID, results[0].SceneID)
	}
	// All hashes have the same delta (100), so they all land in the same bin.
	// span = (numHashes-1) - 0 + 1 = numHashes, score = numHashes/numHashes = 1.0
	if results[0].ConfidenceScore < 0.99 {
		t.Errorf("expected confidence score ~1.0 for constant-offset match, got %f", results[0].ConfidenceScore)
	}
}

func TestFindAudioMatches_BelowMinHashes(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)
	minHashes := 10
	queryHashes := make([]int32, 20)
	hits := make(map[int32][]AudioHit)

	// Only 5 matching hashes (below minHashes=10)
	for i := 0; i < 20; i++ {
		hash := int32(3000 + i)
		queryHashes[i] = hash
		if i < 5 {
			hits[hash] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(i)}}
		}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, minHashes, 0.5, 2, 0)

	if len(results) != 0 {
		t.Fatalf("expected 0 results when below minHashes, got %d: %v", len(results), results)
	}
}

func TestFindAudioMatches_BelowDensityThreshold(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)
	// 10 matching hashes spread across a span of 100 positions
	// density = 10/100 = 0.1, which is below threshold of 0.5
	queryHashes := make([]int32, 100)
	hits := make(map[int32][]AudioHit)

	for i := 0; i < 100; i++ {
		hash := int32(4000 + i)
		queryHashes[i] = hash
	}

	// Place matches at positions 0, 10, 20, ..., 90 (10 hits, span of 91)
	sparsePositions := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90}
	for _, pos := range sparsePositions {
		hash := queryHashes[pos]
		hits[hash] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(pos)}}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, 2, 0)

	// 10 hits in a span of 91, density = 10/91 ~ 0.11 < 0.5
	if len(results) != 0 {
		t.Fatalf("expected 0 results when below density threshold, got %d: %v", len(results), results)
	}
}

func TestFindAudioMatches_MultipleScenes(t *testing.T) {
	querySceneID := uint(1)
	sceneB := uint(2)
	sceneC := uint(3)
	numHashes := 30
	queryHashes := make([]int32, numHashes)
	hits := make(map[int32][]AudioHit)

	for i := 0; i < numHashes; i++ {
		hash := int32(5000 + i)
		queryHashes[i] = hash
		var hashHits []AudioHit

		// Scene B: matches all 30 hashes at same offset (perfect match)
		hashHits = append(hashHits, AudioHit{SceneID: sceneB, Offset: uint32(i)})

		// Scene C: matches only first 15 hashes at same offset
		if i < 15 {
			hashHits = append(hashHits, AudioHit{SceneID: sceneC, Offset: uint32(i)})
		}

		hits[hash] = hashHits
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, 2, 0)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}

	resultMap := make(map[uint]MatchResult)
	for _, r := range results {
		resultMap[r.SceneID] = r
	}

	resB, okB := resultMap[sceneB]
	resC, okC := resultMap[sceneC]
	if !okB {
		t.Fatalf("expected result for scene %d", sceneB)
	}
	if !okC {
		t.Fatalf("expected result for scene %d", sceneC)
	}

	// Scene B: 30/30 = 1.0, Scene C: 15/15 = 1.0
	// Both have perfect density within their matched regions
	if resB.ConfidenceScore < 0.99 {
		t.Errorf("scene B: expected confidence ~1.0, got %f", resB.ConfidenceScore)
	}
	if resC.ConfidenceScore < 0.99 {
		t.Errorf("scene C: expected confidence ~1.0, got %f", resC.ConfidenceScore)
	}
	if resB.MatchType != "audio" || resC.MatchType != "audio" {
		t.Errorf("expected MatchType 'audio' for both, got B=%q C=%q", resB.MatchType, resC.MatchType)
	}
}

func TestFindAudioMatches_DeltaToleranceDefault(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)
	numHashes := 20
	queryHashes := make([]int32, numHashes)
	hits := make(map[int32][]AudioHit)

	// Hashes match at same offset (delta=0), so binKey = 0/2 = 0 with default tolerance
	for i := 0; i < numHashes; i++ {
		hash := int32(6000 + i)
		queryHashes[i] = hash
		hits[hash] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(i)}}
	}

	// deltaTolerance=0 should default to 2
	results := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, 0, 0)

	if len(results) != 1 {
		t.Fatalf("expected 1 result with default deltaTolerance, got %d", len(results))
	}
	if results[0].SceneID != matchSceneID {
		t.Errorf("expected SceneID %d, got %d", matchSceneID, results[0].SceneID)
	}
	if results[0].ConfidenceScore < 0.99 {
		t.Errorf("expected confidence ~1.0, got %f", results[0].ConfidenceScore)
	}

	// Also test negative deltaTolerance
	resultsNeg := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, -5, 0)

	if len(resultsNeg) != 1 {
		t.Fatalf("expected 1 result with negative deltaTolerance (defaults to 2), got %d", len(resultsNeg))
	}
	if resultsNeg[0].ConfidenceScore < 0.99 {
		t.Errorf("expected confidence ~1.0 with negative deltaTolerance, got %f", resultsNeg[0].ConfidenceScore)
	}
}

func TestFindAudioMatches_MultipleHitsPerPosition(t *testing.T) {
	// Verify that multiple ClickHouse hits for the same query position don't
	// inflate the density score above 1.0. This was the root cause of false
	// positives: a hash appearing at multiple offsets in a candidate scene
	// caused the count to exceed the span, producing scores like 200-500%.
	querySceneID := uint(1)
	matchSceneID := uint(2)
	numHashes := 20
	queryHashes := make([]int32, numHashes)
	hits := make(map[int32][]AudioHit)

	for i := 0; i < numHashes; i++ {
		hash := int32(8000 + i)
		queryHashes[i] = hash
		// Each hash has 5 hits from the same scene at different offsets.
		// With deltaTolerance=2, some of these may land in the same bin.
		hits[hash] = []AudioHit{
			{SceneID: matchSceneID, Offset: uint32(i)},
			{SceneID: matchSceneID, Offset: uint32(i + 1000)},
			{SceneID: matchSceneID, Offset: uint32(i + 2000)},
			{SceneID: matchSceneID, Offset: uint32(i + 3000)},
			{SceneID: matchSceneID, Offset: uint32(i + 4000)},
		}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 5, 0.5, 2, 0)

	// Should still find matches, but score must not exceed 1.0
	for _, r := range results {
		if r.ConfidenceScore > 1.0 {
			t.Errorf("confidence score should not exceed 1.0, got %f for scene %d", r.ConfidenceScore, r.SceneID)
		}
	}
	// The best bin should have score ~1.0 (all 20 query positions hit at delta=0)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].ConfidenceScore < 0.99 {
		t.Errorf("expected confidence score ~1.0, got %f", results[0].ConfidenceScore)
	}
}

func TestFindAudioMatches_SubsetMatch(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)
	// Query has 100 hashes, but scene B only matches a dense region of 20
	queryHashes := make([]int32, 100)
	hits := make(map[int32][]AudioHit)

	for i := 0; i < 100; i++ {
		hash := int32(7000 + i)
		queryHashes[i] = hash
	}

	// Scene B matches positions 40-59 (20 consecutive hashes, same offset)
	for i := 40; i < 60; i++ {
		hash := queryHashes[i]
		hits[hash] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(i)}}
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, 10, 0.8, 2, 0)

	if len(results) != 1 {
		t.Fatalf("expected 1 result for subset/clip match, got %d", len(results))
	}
	if results[0].SceneID != matchSceneID {
		t.Errorf("expected SceneID %d, got %d", matchSceneID, results[0].SceneID)
	}
	// 20 hits in span of 20, score = 20/20 = 1.0
	if results[0].ConfidenceScore < 0.99 {
		t.Errorf("expected confidence ~1.0 for dense subset match, got %f", results[0].ConfidenceScore)
	}
	if results[0].MatchType != "audio" {
		t.Errorf("expected MatchType 'audio', got %q", results[0].MatchType)
	}
}

func TestFindAudioMatches_BestBinByDensity(t *testing.T) {
	// Verify that the best bin is selected by density score, not raw count.
	// Scene B has two bins:
	//   Bin A: 200 positions in span 1000 -> density 0.2 (rejected)
	//   Bin B: 100 positions in span 120  -> density 0.83 (should be selected)
	// The old logic picked Bin A (highest count) then rejected it for low density,
	// missing Bin B entirely. The new logic evaluates all qualifying bins.
	querySceneID := uint(1)
	matchSceneID := uint(2)
	deltaTolerance := 2
	minHashes := 50
	densityThreshold := 0.5
	minSpan := 0

	// We need enough query hashes to cover both bins' query positions.
	// Bin A uses positions 0-999, Bin B uses positions 1100-1219.
	queryHashes := make([]int32, 1300)
	for i := range queryHashes {
		queryHashes[i] = int32(20000 + i)
	}

	hits := make(map[int32][]AudioHit)

	// Bin A: 200 sparse matches across positions 0-999.
	// With deltaTolerance=2 and delta=500, binKey = 500/2 = 250.
	// Place 200 hits at every 5th position (0, 5, 10, ..., 995).
	for i := 0; i < 200; i++ {
		pos := i * 5
		hash := queryHashes[pos]
		hits[hash] = append(hits[hash], AudioHit{
			SceneID: matchSceneID,
			Offset:  uint32(pos + 500), // delta = 500
		})
	}

	// Bin B: 100 dense matches across a span of 120 (positions 1100-1219).
	// With deltaTolerance=2 and delta=2000, binKey = 2000/2 = 1000.
	// We place 100 hits within positions 1100-1219 to get density 100/120 = 0.833.
	binBPositions := make([]int, 0, 100)
	binBPositions = append(binBPositions, 1100) // min
	binBPositions = append(binBPositions, 1219) // max (sets span to 120)
	for i := 1; i <= 98; i++ {
		binBPositions = append(binBPositions, 1100+i)
	}
	for _, pos := range binBPositions {
		hash := queryHashes[pos]
		hits[hash] = append(hits[hash], AudioHit{
			SceneID: matchSceneID,
			Offset:  uint32(pos + 2000), // delta = 2000
		})
	}

	results := FindAudioMatches(querySceneID, queryHashes, hits, minHashes, densityThreshold, deltaTolerance, minSpan)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SceneID != matchSceneID {
		t.Fatalf("expected SceneID %d, got %d", matchSceneID, results[0].SceneID)
	}
	// Bin B: 100/120 = 0.833, Bin A: 200/996 = 0.2 (below threshold, not selected)
	// Best score should be ~0.83 from Bin B
	if results[0].ConfidenceScore < 0.80 {
		t.Errorf("expected confidence score >= 0.80 (from dense bin), got %f", results[0].ConfidenceScore)
	}
	if results[0].ConfidenceScore > 0.90 {
		t.Errorf("expected confidence score <= 0.90 (from dense bin), got %f", results[0].ConfidenceScore)
	}
}

func TestFindAudioMatches_MinSpan(t *testing.T) {
	querySceneID := uint(1)
	matchSceneID := uint(2)

	// Create a match with 20 hashes in a span of 20 (positions 0-19).
	// This represents ~2.5 seconds of audio at 8 hashes/sec.
	queryHashes := make([]int32, 100)
	hits := make(map[int32][]AudioHit)
	for i := 0; i < 100; i++ {
		hash := int32(10000 + i)
		queryHashes[i] = hash
	}
	for i := 0; i < 20; i++ {
		hits[queryHashes[i]] = []AudioHit{{SceneID: matchSceneID, Offset: uint32(i)}}
	}

	// Without minSpan: should pass (20 hashes, 20 span, density=1.0)
	results := FindAudioMatches(querySceneID, queryHashes, hits, 10, 0.5, 2, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result without minSpan, got %d", len(results))
	}

	// With minSpan=160 (~20 seconds): span of 20 < 160, should be rejected
	resultsFiltered := FindAudioMatches(querySceneID, queryHashes, hits, 10, 0.5, 2, 160)
	if len(resultsFiltered) != 0 {
		t.Fatalf("expected 0 results with minSpan=160 (span=20), got %d", len(resultsFiltered))
	}

	// With minSpan=15: span of 20 >= 15, should pass
	resultsPass := FindAudioMatches(querySceneID, queryHashes, hits, 10, 0.5, 2, 15)
	if len(resultsPass) != 1 {
		t.Fatalf("expected 1 result with minSpan=15 (span=20), got %d", len(resultsPass))
	}
}

func TestFloorDiv(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{7, 3, 2},      // 7/3 = 2.33, floor = 2
		{-7, 3, -3},    // -7/3 = -2.33, floor = -3 (Go truncation gives -2)
		{7, -3, -3},    // 7/-3 = -2.33, floor = -3 (Go truncation gives -2)
		{-7, -3, 2},    // -7/-3 = 2.33, floor = 2
		{6, 3, 2},      // exact division
		{-6, 3, -2},    // exact division
		{0, 3, 0},      // zero dividend
		{1, 2, 0},      // 1/2 = 0.5, floor = 0
		{-1, 2, -1},    // -1/2 = -0.5, floor = -1 (Go truncation gives 0)
		{3, 2, 1},      // 3/2 = 1.5, floor = 1
		{-3, 2, -2},    // -3/2 = -1.5, floor = -2 (Go truncation gives -1)
	}

	for _, tt := range tests {
		got := floorDiv(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("floorDiv(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestFloorDiv_BinningSymmetry(t *testing.T) {
	// Verify that floorDiv produces uniform bin widths around zero.
	// With deltaTolerance=2:
	// - bin -1 should cover deltas {-2, -1}
	// - bin 0 should cover deltas {0, 1}
	// - bin 1 should cover deltas {2, 3}
	// Without floorDiv (using Go truncation), bin 0 would cover {-1, 0, 1} (3 values).
	deltaTolerance := 2

	binCounts := make(map[int]int)
	// Use an even range so bins divide evenly (20 values = 10 bins of 2)
	for delta := -10; delta < 10; delta++ {
		bin := floorDiv(delta, deltaTolerance)
		binCounts[bin]++
	}

	// Every bin should have exactly 2 members (deltaTolerance=2)
	for bin, count := range binCounts {
		if count != deltaTolerance {
			t.Errorf("bin %d has %d members, expected %d", bin, count, deltaTolerance)
		}
	}
}
