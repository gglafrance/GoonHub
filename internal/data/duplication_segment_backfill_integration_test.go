package data

import (
	"fmt"
	"testing"

	"github.com/lib/pq"
)

// Integration test of ListMembersWithoutSegment (segment backfill): only the members without
// a segment are listed, by ascending ID after the cursor. Runs on the disposable import test
// database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDuplicateGroupRepositoryMembersWithoutSegment(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewDuplicateGroupRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	newScene := func(name string) *Scene {
		s := &Scene{Title: name, Duration: 600, StoredPath: fmt.Sprintf("/import-test/%s/%s.mp4", suffix, name), Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	a, b, c := newScene("seg-a"), newScene("seg-b"), newScene("seg-c")
	g := &DuplicateGroup{Status: "unresolved", SceneCount: 3}
	if err := repo.Create(g); err != nil {
		t.Fatalf("create group: %v", err)
	}
	t.Cleanup(func() { db.Delete(&DuplicateGroup{}, g.ID) })
	start, end := 10.0, 400.0
	members := []*DuplicateGroupMember{
		{GroupID: g.ID, SceneID: a.ID, MatchType: "visual"},
		{GroupID: g.ID, SceneID: b.ID, MatchType: "audio", MatchStart: &start, MatchEnd: &end, RefSceneID: &a.ID, RefStart: &start, RefEnd: &end},
		{GroupID: g.ID, SceneID: c.ID, MatchType: "audio"},
	}
	for _, m := range members {
		if err := repo.AddMember(m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	var got []uint
	var after uint
	for {
		rows, err := repo.ListMembersWithoutSegment(after, 1)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			after = r.ID
			if r.GroupID == g.ID {
				got = append(got, r.SceneID)
			}
		}
	}
	if len(got) != 2 || got[0] != a.ID || got[1] != c.ID {
		t.Fatalf("members without segment = %v, want [%d %d]", got, a.ID, c.ID)
	}
}
