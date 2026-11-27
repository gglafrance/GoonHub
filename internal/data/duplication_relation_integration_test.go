package data

import (
	"fmt"
	"testing"

	"github.com/lib/pq"
)

// Integration test of the duplicate relation queries (migration 000085): the kind filter of
// ListWithMembers, the relation written by FillMemberSegment and the startup backfill queries.
// Runs on the disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDuplicateGroupRepositoryRelations(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewDuplicateGroupRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	newScene := func(name string, duration int) *Scene {
		s := &Scene{Title: name, Duration: duration, StoredPath: fmt.Sprintf("/import-test/%s/%s.mp4", suffix, name), Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	newGroup := func(members ...*DuplicateGroupMember) *DuplicateGroup {
		g := &DuplicateGroup{Status: "unresolved", SceneCount: len(members)}
		if err := repo.Create(g); err != nil {
			t.Fatalf("create group: %v", err)
		}
		t.Cleanup(func() { db.Delete(&DuplicateGroup{}, g.ID) })
		for _, m := range members {
			m.GroupID = g.ID
			m.MatchType = "audio"
			if err := repo.AddMember(m); err != nil {
				t.Fatalf("add member: %v", err)
			}
		}
		return g
	}
	f64 := func(v float64) *float64 { return &v }

	a, b := newScene("ident-a", 600), newScene("ident-b", 600)
	clip, comp := newScene("clip", 1642), newScene("compilation", 5627)
	p1, p2 := newScene("partial-a", 600), newScene("partial-b", 600)
	u1, u2 := newScene("unverified-a", 600), newScene("unverified-b", 600)

	identical := newGroup(
		&DuplicateGroupMember{SceneID: a.ID, Relation: DuplicateRelationIdentical},
		&DuplicateGroupMember{SceneID: b.ID, Relation: DuplicateRelationIdentical},
	)
	containment := newGroup(
		&DuplicateGroupMember{SceneID: clip.ID, Relation: DuplicateRelationContained},
		&DuplicateGroupMember{SceneID: comp.ID, Relation: DuplicateRelationContains},
	)
	partial := newGroup(
		&DuplicateGroupMember{SceneID: p1.ID, Relation: DuplicateRelationPartial},
		&DuplicateGroupMember{SceneID: p2.ID, Relation: DuplicateRelationIdentical},
	)
	// No relation given: the column default (unknown) applies; u2 has a segment to backfill
	unverified := newGroup(
		&DuplicateGroupMember{SceneID: u1.ID, Relation: DuplicateRelationIdentical},
		&DuplicateGroupMember{SceneID: u2.ID, MatchStart: f64(0), MatchEnd: f64(598), RefSceneID: &u1.ID, RefStart: f64(1), RefEnd: f64(599)},
	)

	ids := func(kind string) map[uint]bool {
		groups, total, err := repo.ListWithMembers(1, 1000, "unresolved", kind)
		if err != nil {
			t.Fatalf("ListWithMembers(%q): %v", kind, err)
		}
		if int(total) != len(groups) {
			t.Fatalf("ListWithMembers(%q): total %d for %d groups", kind, total, len(groups))
		}
		result := map[uint]bool{}
		for _, g := range groups {
			result[g.ID] = true
		}
		return result
	}
	all := []*DuplicateGroup{identical, containment, partial, unverified}
	for _, tt := range []struct {
		kind string
		want *DuplicateGroup
	}{
		{DuplicateKindIdentical, identical},
		{DuplicateKindContainment, containment},
		{DuplicateKindPartial, partial},
		{DuplicateKindUnverified, unverified},
	} {
		got := ids(tt.kind)
		for _, g := range all {
			if got[g.ID] != (g == tt.want) {
				t.Fatalf("kind %q: group %d listed=%t", tt.kind, g.ID, got[g.ID])
			}
		}
	}
	every := ids("")
	for _, g := range all {
		if !every[g.ID] {
			t.Fatalf("group %d missing without kind filter", g.ID)
		}
	}

	// Backfill: u2 is listed with the durations of its scene and reference
	var found *DuplicateMemberRelationInput
	var afterID uint
	for found == nil {
		rows, err := repo.ListMembersForRelationBackfill(afterID, 100)
		if err != nil {
			t.Fatalf("ListMembersForRelationBackfill: %v", err)
		}
		for i := range rows {
			if rows[i].ID <= afterID {
				t.Fatalf("rows not in ascending order after %d: %+v", afterID, rows)
			}
			afterID = rows[i].ID
			var member DuplicateGroupMember
			if err := db.First(&member, rows[i].ID).Error; err != nil {
				t.Fatalf("load member: %v", err)
			}
			if member.SceneID == u2.ID {
				found = &rows[i]
			}
			if member.SceneID == u1.ID || member.SceneID == clip.ID {
				t.Fatalf("member without segment or already classified listed: %+v", member)
			}
		}
		if len(rows) < 100 {
			break
		}
	}
	if found == nil || found.Duration != 600 || found.RefDuration != 600 || found.MatchEnd == nil || *found.MatchEnd != 598 {
		t.Fatalf("unverified member not listed for backfill: %+v", found)
	}
	if err := repo.SetMemberRelation(found.ID, DuplicateRelationIdentical); err != nil {
		t.Fatalf("SetMemberRelation: %v", err)
	}
	if got := ids(DuplicateKindIdentical); !got[unverified.ID] {
		t.Fatal("group with every member classified identical not listed as identical")
	}

	// FillMemberSegment writes the relation along with a missing segment, only once
	seg := &DuplicateGroupMember{GroupID: partial.ID, SceneID: p1.ID, MatchStart: f64(0), MatchEnd: f64(100),
		RefSceneID: &p2.ID, RefStart: f64(0), RefEnd: f64(100), Relation: DuplicateRelationContained}
	if err := repo.FillMemberSegment(seg); err != nil {
		t.Fatalf("FillMemberSegment: %v", err)
	}
	again := *seg
	again.Relation = DuplicateRelationIdentical
	if err := repo.FillMemberSegment(&again); err != nil {
		t.Fatalf("FillMemberSegment: %v", err)
	}
	g, err := repo.GetByIDWithMembers(partial.ID)
	if err != nil {
		t.Fatalf("GetByIDWithMembers: %v", err)
	}
	for _, m := range g.Members {
		if m.SceneID == p1.ID && (m.Relation != DuplicateRelationContained || m.MatchEnd == nil || *m.MatchEnd != 100) {
			t.Fatalf("segment relation not stored once: %+v", m)
		}
	}

	// The CHECK constraint refuses unknown relation values
	if err := db.Exec(`UPDATE duplicate_group_members SET relation = 'bogus' WHERE group_id = ?`, partial.ID).Error; err == nil {
		t.Fatal("expected the relation CHECK constraint to refuse 'bogus'")
	}
}
