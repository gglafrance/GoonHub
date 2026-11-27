package data

import (
	"fmt"
	"testing"

	"github.com/lib/pq"
)

// Integration test of the duplicate exclusions (POST /admin/duplicates/:id/exclude):
// AddExclusions stores each pair once, ordered, and is idempotent; ListExclusionsAmong only
// returns the pairs whose two scenes are in the set; RemoveMembers removes the given members;
// a rolled back transaction leaves nothing. Runs on the disposable import test database
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDuplicateGroupRepositoryExclusions(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewDuplicateGroupRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	scenes := make([]uint, 4)
	for i := range scenes {
		s := &Scene{Title: fmt.Sprintf("excl-%d", i), Duration: 600, StoredPath: fmt.Sprintf("/import-test/%s/excl-%d.mp4", suffix, i), Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		scenes[i] = s.ID
	}
	a, b, c, d := scenes[0], scenes[1], scenes[2], scenes[3]
	t.Cleanup(func() { db.Where("scene_id_a IN ? OR scene_id_b IN ?", scenes, scenes).Delete(&DuplicateExclusion{}) })

	group := &DuplicateGroup{Status: "unresolved", SceneCount: 4}
	if err := repo.Create(group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	t.Cleanup(func() { db.Delete(&DuplicateGroup{}, group.ID) })
	for _, id := range scenes {
		if err := repo.AddMember(&DuplicateGroupMember{GroupID: group.ID, SceneID: id, MatchType: "audio"}); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	// The highest ID excluded from the others, twice, with itself in the other list
	if err := repo.AddExclusions([]uint{d}, []uint{a, b, d}); err != nil {
		t.Fatalf("add exclusions: %v", err)
	}
	if err := repo.AddExclusions([]uint{d}, []uint{b, a}); err != nil {
		t.Fatalf("add exclusions again: %v", err)
	}
	rows, err := repo.ListExclusionsAmong(scenes)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 pairs, got %+v", rows)
	}
	for _, r := range rows {
		if r.SceneIDA >= r.SceneIDB || r.SceneIDB != d || r.CreatedAt.IsZero() {
			t.Fatalf("pair not ordered or incomplete: %+v", r)
		}
	}
	if rows, err := repo.ListExclusionsAmong([]uint{a, b, c}); err != nil || len(rows) != 0 {
		t.Fatalf("pairs with a scene outside the set must not be listed: %+v (%v)", rows, err)
	}
	if rows, err := repo.ListExclusionsAmong([]uint{b, d}); err != nil || len(rows) != 1 || rows[0].SceneIDA != b {
		t.Fatalf("expected the pair (b, d): %+v (%v)", rows, err)
	}

	// A rolled back transaction records nothing
	_ = repo.RunInTransaction(func(tx DuplicateGroupRepository) error {
		if err := tx.AddExclusions([]uint{c}, []uint{a}); err != nil {
			t.Fatalf("add in transaction: %v", err)
		}
		return fmt.Errorf("roll back")
	})
	if rows, _ := repo.ListExclusionsAmong([]uint{a, c}); len(rows) != 0 {
		t.Fatalf("rolled back exclusion was kept: %+v", rows)
	}

	if err := repo.RemoveMembers(group.ID, []uint{c, d}); err != nil {
		t.Fatalf("remove members: %v", err)
	}
	if err := repo.UpdateSceneCount(group.ID); err != nil {
		t.Fatalf("update scene count: %v", err)
	}
	got, err := repo.GetByIDWithMembers(group.ID)
	if err != nil {
		t.Fatalf("get group: %v", err)
	}
	if got.SceneCount != 2 || len(got.Members) != 2 {
		t.Fatalf("expected 2 members left, got %+v", got)
	}
	for _, m := range got.Members {
		if m.SceneID == c || m.SceneID == d {
			t.Fatalf("scene %d still a member", m.SceneID)
		}
	}
}
