package core

import (
	"math"
	"strings"
	"testing"

	"goonhub/internal/data"
)

func seg(v float64) *float64 { return &v }

func TestClassifyDuplicateRelation(t *testing.T) {
	tests := []struct {
		name string
		in   DuplicateRelationInput
		want string
	}{
		{
			// Group 64: scene 4448 (1,642 s clip) found inside scene 4446 (5,627 s compilation)
			name: "clip of a compilation (4448 in 4446)",
			in:   DuplicateRelationInput{MatchStart: seg(1.5), MatchEnd: seg(1640), RefStart: seg(2101.5), RefEnd: seg(3740), Duration: 1642, RefDuration: 5627},
			want: data.DuplicateRelationContained,
		},
		{
			name: "compilation containing a clip (4446 contains 4448)",
			in:   DuplicateRelationInput{MatchStart: seg(2101.5), MatchEnd: seg(3740), RefStart: seg(1.5), RefEnd: seg(1640), Duration: 5627, RefDuration: 1642},
			want: data.DuplicateRelationContains,
		},
		{
			name: "same video, equal durations",
			in:   DuplicateRelationInput{MatchStart: seg(2), MatchEnd: seg(598), RefStart: seg(2), RefEnd: seg(598), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationIdentical,
		},
		{
			name: "same video, one with a 30 s intro",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(1795), RefStart: seg(30), RefEnd: seg(1825), Duration: 1800, RefDuration: 1830},
			want: data.DuplicateRelationIdentical,
		},
		{
			name: "short scenes, 40 s left out (83 %) within the slack",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(200), RefStart: seg(0), RefEnd: seg(200), Duration: 240, RefDuration: 240},
			want: data.DuplicateRelationIdentical,
		},
		{
			name: "very short scenes, half matched: slack needs 75 %",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(30), RefStart: seg(0), RefEnd: seg(30), Duration: 60, RefDuration: 60},
			want: data.DuplicateRelationPartial,
		},
		{
			name: "equal durations, half matched",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(300), RefStart: seg(300), RefEnd: seg(600), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationPartial,
		},
		{
			name: "short cut of an extended version",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(1500), RefStart: seg(0), RefEnd: seg(1500), Duration: 1500, RefDuration: 1800},
			want: data.DuplicateRelationContained,
		},
		{
			name: "exactly 90 % of long scenes",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(900), RefStart: seg(0), RefEnd: seg(900), Duration: 1000, RefDuration: 1000},
			want: data.DuplicateRelationIdentical,
		},
		{
			name: "just under 90 % of long scenes",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(899), RefStart: seg(0), RefEnd: seg(899), Duration: 1000, RefDuration: 1000},
			want: data.DuplicateRelationPartial,
		},
		{
			name: "segment end past the duration is clamped",
			in:   DuplicateRelationInput{MatchStart: seg(-1), MatchEnd: seg(612), RefStart: seg(0), RefEnd: seg(601.4), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationIdentical,
		},
		{
			name: "missing member duration",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(600), RefStart: seg(0), RefEnd: seg(600), Duration: 0, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "missing reference duration",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(600), RefStart: seg(0), RefEnd: seg(600), Duration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "no segment yet (older group)",
			in:   DuplicateRelationInput{Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "reference segment missing",
			in:   DuplicateRelationInput{MatchStart: seg(0), MatchEnd: seg(600), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "zero-length segment",
			in:   DuplicateRelationInput{MatchStart: seg(100), MatchEnd: seg(100), RefStart: seg(100), RefEnd: seg(100), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "inverted segment",
			in:   DuplicateRelationInput{MatchStart: seg(500), MatchEnd: seg(100), RefStart: seg(0), RefEnd: seg(600), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "segment outside the scene",
			in:   DuplicateRelationInput{MatchStart: seg(700), MatchEnd: seg(800), RefStart: seg(0), RefEnd: seg(100), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
		{
			name: "not a number",
			in:   DuplicateRelationInput{MatchStart: seg(math.NaN()), MatchEnd: seg(600), RefStart: seg(0), RefEnd: seg(600), Duration: 600, RefDuration: 600},
			want: data.DuplicateRelationUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyDuplicateRelation(tt.in); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDuplicateGroupKind(t *testing.T) {
	const (
		identical = data.DuplicateRelationIdentical
		contained = data.DuplicateRelationContained
		contains  = data.DuplicateRelationContains
		partial   = data.DuplicateRelationPartial
		unknown   = data.DuplicateRelationUnknown
	)
	tests := []struct {
		relations []string
		want      string
	}{
		{[]string{identical, identical}, data.DuplicateKindIdentical},
		{[]string{contained, contains}, data.DuplicateKindContainment},
		{[]string{identical, identical, contained}, data.DuplicateKindContainment},
		{[]string{partial, unknown, contains}, data.DuplicateKindContainment},
		{[]string{identical, partial}, data.DuplicateKindPartial},
		{[]string{partial, unknown}, data.DuplicateKindPartial},
		{[]string{identical, unknown}, data.DuplicateKindUnverified},
		{[]string{identical, ""}, data.DuplicateKindUnverified},
		{nil, data.DuplicateKindUnverified},
	}
	for _, tt := range tests {
		if got := DuplicateGroupKind(tt.relations); got != tt.want {
			t.Fatalf("%v: got %q, want %q", tt.relations, got, tt.want)
		}
	}
}

func TestAutomaticResolutionBlocker(t *testing.T) {
	id4446, id4448 := uint(4446), uint(4448)
	group := &data.DuplicateGroup{ID: 64, Members: []data.DuplicateGroupMember{
		{SceneID: 4446, RefSceneID: &id4448, Relation: data.DuplicateRelationContains},
		{SceneID: 4448, RefSceneID: &id4446, Relation: data.DuplicateRelationContained},
	}}
	blocker := automaticResolutionBlocker(group)
	if blocker == nil {
		t.Fatal("a clip and its compilation must not be resolved automatically")
	}
	if blocker.GroupID != 64 || blocker.Kind != data.DuplicateKindContainment || len(blocker.Members) != 2 {
		t.Fatalf("unexpected blocker: %+v", blocker)
	}
	for _, want := range []string{"#4446 contains #4448", "#4448 is a clip of #4446"} {
		if !strings.Contains(blocker.Reason, want) {
			t.Fatalf("reason %q does not mention %q", blocker.Reason, want)
		}
	}

	// Unverified members block too, and only the non-identical members are listed
	group = &data.DuplicateGroup{ID: 3, Members: []data.DuplicateGroupMember{
		{SceneID: 1, Relation: data.DuplicateRelationIdentical},
		{SceneID: 2},
	}}
	blocker = automaticResolutionBlocker(group)
	if blocker == nil || blocker.Kind != data.DuplicateKindUnverified || len(blocker.Members) != 1 ||
		blocker.Members[0].SceneID != 2 || blocker.Members[0].Relation != data.DuplicateRelationUnknown {
		t.Fatalf("unexpected blocker: %+v", blocker)
	}

	group = &data.DuplicateGroup{ID: 4, Members: []data.DuplicateGroupMember{
		{SceneID: 1, Relation: data.DuplicateRelationIdentical},
		{SceneID: 2, Relation: data.DuplicateRelationIdentical},
	}}
	if blocker := automaticResolutionBlocker(group); blocker != nil {
		t.Fatalf("identical group blocked: %+v", blocker)
	}
}
