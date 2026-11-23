package data

import (
	"fmt"
	"testing"

	"github.com/lib/pq"
)

// Integration test of ListUnresolvedIDs ("select all" of the duplicates page): only unresolved
// groups, the kind filter, the limit and the total. Runs on the disposable import test
// database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDuplicateGroupRepositoryListUnresolvedIDs(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewDuplicateGroupRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	// Other tests share this database: count from a clean slate of unresolved groups.
	_, before, err := repo.ListUnresolvedIDs("", 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	_, identicalBefore, err := repo.ListUnresolvedIDs(DuplicateKindIdentical, 1)
	if err != nil {
		t.Fatalf("list identical: %v", err)
	}

	n := 0
	newGroup := func(status, relation string) *DuplicateGroup {
		g := &DuplicateGroup{Status: status, SceneCount: 2}
		if err := repo.Create(g); err != nil {
			t.Fatalf("create group: %v", err)
		}
		t.Cleanup(func() { db.Delete(&DuplicateGroup{}, g.ID) })
		for i := 0; i < 2; i++ {
			n++
			s := &Scene{Title: fmt.Sprintf("sel-%d", n), Duration: 600, StoredPath: fmt.Sprintf("/import-test/%s/sel-%d.mp4", suffix, n), Tags: pq.StringArray{}, Actors: pq.StringArray{}}
			if err := db.Create(s).Error; err != nil {
				t.Fatalf("create scene: %v", err)
			}
			if err := repo.AddMember(&DuplicateGroupMember{GroupID: g.ID, SceneID: s.ID, MatchType: "audio", Relation: relation}); err != nil {
				t.Fatalf("add member: %v", err)
			}
		}
		return g
	}
	identical := newGroup("unresolved", DuplicateRelationIdentical)
	partial := newGroup("unresolved", DuplicateRelationPartial)
	newGroup("resolved", DuplicateRelationIdentical)
	newGroup("dismissed", DuplicateRelationIdentical)

	ids, total, err := repo.ListUnresolvedIDs("", 1000)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != before+2 {
		t.Fatalf("total = %d, want %d (only the unresolved groups)", total, before+2)
	}
	seen := map[uint]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen[identical.ID] || !seen[partial.ID] {
		t.Fatalf("unresolved groups missing from %v", ids)
	}

	ids, total, err = repo.ListUnresolvedIDs(DuplicateKindIdentical, 1000)
	if err != nil {
		t.Fatalf("list identical: %v", err)
	}
	if total != identicalBefore+1 {
		t.Fatalf("identical total = %d, want %d", total, identicalBefore+1)
	}
	for _, id := range ids {
		if id == partial.ID {
			t.Fatal("the partial group is listed under the identical kind")
		}
	}

	ids, total, err = repo.ListUnresolvedIDs("", 1)
	if err != nil {
		t.Fatalf("list limited: %v", err)
	}
	if len(ids) != 1 || total != before+2 {
		t.Fatalf("limit 1: got %d ids, total %d", len(ids), total)
	}
}
