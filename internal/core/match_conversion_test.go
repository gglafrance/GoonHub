package core

import (
	"testing"

	"goonhub/internal/core/matching"
	"goonhub/internal/data"
)

// The aligned segment must survive the round trip matcher -> job result -> matcher: losing it
// stored "0 to one fingerprint step" as the matching segment of every duplicate group member.
func TestMatchConversionKeepsTheSegment(t *testing.T) {
	in := []matching.MatchResult{
		{SceneID: 7, ConfidenceScore: 0.93, MatchType: "visual", QueryStart: 12, QueryEnd: 140, Delta: -5},
		{SceneID: 9, ConfidenceScore: 0.71, MatchType: "audio", QueryStart: 300, QueryEnd: 2100, Delta: 42},
	}
	out := toMatchingMatches(toProcessingMatches(in))
	if len(out) != len(in) {
		t.Fatalf("got %d matches, want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("match %d: got %+v, want %+v", i, out[i], in[i])
		}
	}

	// And the stored segment is the real one, in seconds (visual: 2 s per position).
	member := &data.DuplicateGroupMember{GroupID: 1, SceneID: 3}
	setMemberSegment(member, out[0], 3, true)
	if *member.MatchStart != 24 || *member.MatchEnd != 282 {
		t.Fatalf("query segment = %v-%v, want 24-282", *member.MatchStart, *member.MatchEnd)
	}
	if *member.RefStart != 14 || *member.RefEnd != 272 {
		t.Fatalf("reference segment = %v-%v, want 14-272", *member.RefStart, *member.RefEnd)
	}
}
