package core

import (
	"slices"
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestMarkerShiftPlace(t *testing.T) {
	// The trashed scene is a 90 s clip found 70 s into the kept 600 s compilation
	clip := markerShift{segment: &SimilarSegment{ThisStart: 70, ThisEnd: 160, OtherStart: 0, OtherEnd: 90}, keptDuration: 600}
	// Identical copies, the trashed one has 5 s less at the start
	identical := markerShift{segment: &SimilarSegment{ThisStart: 5, ThisEnd: 595, OtherStart: 0, OtherEnd: 590}, identical: true, keptDuration: 600}
	// The kept scene starts later in the trashed one: shifted times go below 0 at the edge
	trimmed := markerShift{segment: &SimilarSegment{ThisStart: 0, ThisEnd: 90, OtherStart: 10, OtherEnd: 100}, keptDuration: 90}

	cases := []struct {
		name    string
		shift   markerShift
		start   int
		end     *int
		keep    bool
		ts      int
		wantEnd *int
		shifted bool
	}{
		{"clip: point inside the segment is shifted", clip, 10, nil, true, 80, nil, true},
		{"clip: range inside is shifted", clip, 10, ptr(30), true, 80, ptr(100), true},
		{"clip: end past the segment is clamped to it", clip, 80, ptr(120), true, 150, ptr(160), true},
		{"clip: within the edge tolerance", clip, 91, nil, true, 161, nil, true},
		{"clip: outside the segment is left behind", clip, 95, nil, false, 0, nil, false},
		{"clip: end clamped onto the start is dropped", clip, 90, ptr(100), true, 160, nil, true},
		{"identical: outside the segment but inside the kept scene", identical, 595, nil, true, 600, nil, true},
		{"identical: shifted past the kept scene is left behind", identical, 597, nil, false, 0, nil, false},
		{"identical: end clamped to the kept scene only", identical, 100, ptr(598), true, 105, ptr(600), true},
		{"identical: end overflowing the segment is not clamped to it", identical, 500, ptr(594), true, 505, ptr(599), true},
		{"trimmed: inside within tolerance, clamped to 0", trimmed, 9, nil, true, 0, nil, true},
		{"trimmed: before the segment is left behind", trimmed, 5, nil, false, 0, nil, false},
		{"no segment, identical: offset 0", markerShift{identical: true, keptDuration: 300}, 10, ptr(20), true, 10, ptr(20), false},
		{"no segment, identical: past the kept scene is left behind", markerShift{identical: true, keptDuration: 300}, 305, nil, false, 0, nil, false},
		{"no segment, identical, unknown duration: kept", markerShift{identical: true}, 305, nil, true, 305, nil, false},
		{"no segment, identical: end clamped to the kept scene", markerShift{identical: true, keptDuration: 300}, 290, ptr(310), true, 290, ptr(300), true},
		{"no segment, not identical: left behind", markerShift{keptDuration: 300}, 10, nil, false, 0, nil, false},
		{"partial segment: inside is shifted", markerShift{segment: &SimilarSegment{ThisStart: 200, ThisEnd: 260, OtherStart: 30, OtherEnd: 90}, keptDuration: 900}, 60, nil, true, 230, nil, true},
		{"fractional offset rounds", markerShift{segment: &SimilarSegment{ThisStart: 10.6, ThisEnd: 100, OtherStart: 0, OtherEnd: 89.4}, keptDuration: 100}, 20, nil, true, 31, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.shift.place(tc.start, tc.end)
			if p.keep != tc.keep {
				t.Fatalf("keep = %v (reason %q), want %v", p.keep, p.reason, tc.keep)
			}
			if !p.keep {
				if p.reason == "" {
					t.Fatal("a marker left behind needs a reason")
				}
				return
			}
			if p.timestamp != tc.ts {
				t.Fatalf("timestamp = %d, want %d", p.timestamp, tc.ts)
			}
			if (p.end == nil) != (tc.wantEnd == nil) || (p.end != nil && *p.end != *tc.wantEnd) {
				t.Fatalf("end = %v, want %v", deref(p.end), deref(tc.wantEnd))
			}
			if p.shifted != tc.shifted {
				t.Fatalf("shifted = %v, want %v", p.shifted, tc.shifted)
			}
		})
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestMarkerShiftBetween(t *testing.T) {
	scenes := map[uint]data.Scene{1: {ID: 1, Duration: 600}, 2: {ID: 2, Duration: 90}, 3: {ID: 3, Duration: 600}}

	t.Run("trashed clip matched against the kept compilation", func(t *testing.T) {
		group := &data.DuplicateGroup{ID: 9, Members: []data.DuplicateGroupMember{
			{SceneID: 1},
			{SceneID: 2, RefSceneID: ptr[uint](1), MatchStart: ptr[float64](0), MatchEnd: ptr[float64](90), RefStart: ptr[float64](70), RefEnd: ptr[float64](160), Relation: data.DuplicateRelationContained},
		}}
		s := markerShiftBetween(group, 1, 2, scenes)
		if s.identical || s.keptDuration != 600 || s.segment == nil {
			t.Fatalf("unexpected shift %+v", s)
		}
		if s.segment.ThisStart != 70 || s.segment.OtherStart != 0 || s.segment.ThisEnd != 160 || s.segment.OtherEnd != 90 {
			t.Fatalf("unexpected segment %+v", *s.segment)
		}
	})

	t.Run("kept clip matched against the trashed compilation (inverted)", func(t *testing.T) {
		group := &data.DuplicateGroup{ID: 9, Members: []data.DuplicateGroupMember{
			{SceneID: 2, RefSceneID: ptr[uint](1), MatchStart: ptr[float64](0), MatchEnd: ptr[float64](90), RefStart: ptr[float64](70), RefEnd: ptr[float64](160), Relation: data.DuplicateRelationContained},
			{SceneID: 1},
		}}
		s := markerShiftBetween(group, 2, 1, scenes)
		if s.segment == nil || s.segment.ThisStart != 0 || s.segment.OtherStart != 70 || s.keptDuration != 90 {
			t.Fatalf("unexpected shift %+v", s)
		}
		// 100 s into the compilation is 30 s into the clip
		if p := s.place(100, nil); !p.keep || p.timestamp != 30 {
			t.Fatalf("unexpected placement %+v", p)
		}
	})

	t.Run("both matched against a third scene", func(t *testing.T) {
		group := &data.DuplicateGroup{ID: 9, Members: []data.DuplicateGroupMember{
			{SceneID: 3},
			{SceneID: 1, RefSceneID: ptr[uint](3), MatchStart: ptr[float64](0), MatchEnd: ptr[float64](600), RefStart: ptr[float64](0), RefEnd: ptr[float64](600), Relation: data.DuplicateRelationIdentical},
			{SceneID: 2, RefSceneID: ptr[uint](3), MatchStart: ptr[float64](0), MatchEnd: ptr[float64](90), RefStart: ptr[float64](100), RefEnd: ptr[float64](190), Relation: data.DuplicateRelationContained},
		}}
		s := markerShiftBetween(group, 1, 2, scenes)
		if s.identical || s.segment == nil || s.segment.ThisStart != 100 || s.segment.OtherStart != 0 {
			t.Fatalf("unexpected shift %+v", s)
		}
	})

	t.Run("identical group without segments", func(t *testing.T) {
		group := &data.DuplicateGroup{ID: 9, Members: []data.DuplicateGroupMember{
			{SceneID: 1, RefSceneID: ptr[uint](3), Relation: data.DuplicateRelationIdentical},
			{SceneID: 3, RefSceneID: ptr[uint](1), Relation: data.DuplicateRelationIdentical},
		}}
		s := markerShiftBetween(group, 1, 3, scenes)
		if !s.identical || s.segment != nil {
			t.Fatalf("unexpected shift %+v", s)
		}
	})

	t.Run("unverified pair without segment", func(t *testing.T) {
		group := &data.DuplicateGroup{ID: 9, Members: []data.DuplicateGroupMember{
			{SceneID: 1}, {SceneID: 3, RefSceneID: ptr[uint](1), Relation: data.DuplicateRelationUnknown},
		}}
		s := markerShiftBetween(group, 1, 3, scenes)
		if s.identical || s.segment != nil {
			t.Fatalf("unexpected shift %+v", s)
		}
		if p := s.place(10, nil); p.keep {
			t.Fatal("an unverified pair can't place markers")
		}
	})

	t.Run("no group", func(t *testing.T) {
		s := markerShiftBetween(nil, 1, 2, scenes)
		if s.identical || s.segment != nil || s.keptDuration != 600 {
			t.Fatalf("unexpected shift %+v", s)
		}
	})
}

func TestPlanMarkerDedup(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id, user uint, ts int, label string, opts ...func(*data.MarkerWithTags)) data.MarkerWithTags {
		m := data.MarkerWithTags{UserSceneMarker: data.UserSceneMarker{ID: id, UserID: user, SceneID: 1, Timestamp: ts, Label: label, CreatedAt: t0.Add(time.Duration(id) * time.Minute)}}
		for _, o := range opts {
			o(&m)
		}
		return m
	}
	withEnd := func(e int) func(*data.MarkerWithTags) {
		return func(m *data.MarkerWithTags) { m.EndTimestamp = &e }
	}
	withTags := func(ids ...uint) func(*data.MarkerWithTags) {
		return func(m *data.MarkerWithTags) {
			for _, id := range ids {
				m.Tags = append(m.Tags, data.MarkerTagInfo{ID: id, IsFromLabel: id >= 100})
			}
		}
	}
	type want struct {
		keep    uint
		remove  []uint
		label   string
		addTags []uint
	}
	cases := []struct {
		name    string
		markers []data.MarkerWithTags
		want    []want
	}{
		{"same label within the tolerance, earliest created kept",
			[]data.MarkerWithTags{mk(2, 1, 12, "Kiss"), mk(1, 1, 10, "Kiss")},
			[]want{{keep: 1, remove: []uint{2}, label: "Kiss"}}},
		{"labels differing in case and spacing",
			[]data.MarkerWithTags{mk(1, 1, 10, "Close  up"), mk(2, 1, 11, " close up ")},
			[]want{{keep: 1, remove: []uint{2}, label: "Close  up"}}},
		{"different labels are kept apart",
			[]data.MarkerWithTags{mk(1, 1, 10, "Kiss"), mk(2, 1, 10, "Hug")}, nil},
		{"starts too far apart",
			[]data.MarkerWithTags{mk(1, 1, 10, "Kiss"), mk(2, 1, 13, "Kiss")}, nil},
		{"different users are kept apart",
			[]data.MarkerWithTags{mk(1, 1, 10, "Kiss"), mk(2, 2, 10, "Kiss")}, nil},
		{"the one with an end is kept, and takes the label it lacks",
			[]data.MarkerWithTags{mk(1, 1, 10, "Kiss"), mk(2, 1, 11, "", withEnd(30))},
			[]want{{keep: 2, remove: []uint{1}, label: "Kiss"}}},
		{"more tags wins over creation time, tags are united",
			[]data.MarkerWithTags{mk(1, 1, 10, "Kiss", withTags(5)), mk(2, 1, 10, "Kiss", withTags(6, 100))},
			[]want{{keep: 2, remove: []uint{1}, label: "Kiss", addTags: []uint{5}}}},
		{"a chain is anchored on its first marker",
			[]data.MarkerWithTags{mk(1, 1, 0, "Kiss"), mk(2, 1, 2, "Kiss"), mk(3, 1, 4, "Kiss")},
			[]want{{keep: 1, remove: []uint{2}, label: "Kiss"}}},
		{"an unlabelled set takes the first label and refuses others",
			[]data.MarkerWithTags{mk(1, 1, 10, ""), mk(2, 1, 11, "Kiss"), mk(3, 1, 12, "Hug")},
			[]want{{keep: 2, remove: []uint{1}, label: "Kiss"}}},
		{"three copies folded into one",
			[]data.MarkerWithTags{mk(1, 1, 10, "Kiss", withTags(5)), mk(2, 1, 11, "kiss", withTags(5, 7)), mk(3, 1, 9, "Kiss", withTags(8, 101))},
			[]want{{keep: 2, remove: []uint{3, 1}, label: "kiss", addTags: []uint{8, 101}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planMarkerDedup(tc.markers)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d groups, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				g := got[i]
				var removed, added []uint
				for _, m := range g.remove {
					removed = append(removed, m.ID)
				}
				for _, tag := range g.addTags {
					added = append(added, tag.TagID)
					if tag.IsFromLabel != (tag.TagID >= 100) {
						t.Fatalf("tag %d lost its label flag", tag.TagID)
					}
				}
				if g.keep.ID != w.keep || !slices.Equal(removed, w.remove) || g.label != w.label || !slices.Equal(added, w.addTags) {
					t.Fatalf("group %d = keep %d remove %v label %q tags %v, want %+v", i, g.keep.ID, removed, g.label, added, w)
				}
			}
		})
	}
}
