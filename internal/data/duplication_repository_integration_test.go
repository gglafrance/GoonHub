package data

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of the scheduled deletion queries (and migrations 000068/000069), on the
// disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDuplicateGroupRepositoryScheduledDeletion(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewDuplicateGroupRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	keep := &Scene{Title: "Keep", StoredPath: "/import-test/" + suffix + "/keep.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	drop := &Scene{Title: "Drop", StoredPath: "/import-test/" + suffix + "/drop.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{keep, drop} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}
	group := &DuplicateGroup{Status: "unresolved", SceneCount: 2}
	if err := repo.Create(group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	t.Cleanup(func() { db.Delete(&DuplicateGroup{}, group.ID) })

	start, end := 1.5, 42.0
	refID := keep.ID
	for _, m := range []*DuplicateGroupMember{
		{GroupID: group.ID, SceneID: keep.ID, ConfidenceScore: 1, MatchType: "audio"},
		{GroupID: group.ID, SceneID: drop.ID, ConfidenceScore: 0.9, MatchType: "audio", MatchStart: &start, MatchEnd: &end, RefSceneID: &refID, RefStart: &start, RefEnd: &end},
	} {
		if err := repo.AddMember(m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}

	past := time.Now().Add(-time.Minute)
	if err := repo.SetMemberDeleteAfter(group.ID, []uint{drop.ID}, &past); err != nil {
		t.Fatalf("SetMemberDeleteAfter: %v", err)
	}
	due, err := repo.ListMembersDueForDeletion(time.Now(), 10)
	if err != nil {
		t.Fatalf("ListMembersDueForDeletion: %v", err)
	}
	found := false
	for _, m := range due {
		if m.SceneID == drop.ID {
			found = true
			if m.MatchStart == nil || *m.MatchStart != start || m.RefSceneID == nil || *m.RefSceneID != keep.ID {
				t.Fatalf("segment not stored: %+v", m)
			}
		}
		if m.SceneID == keep.ID {
			t.Fatal("kept scene must not be scheduled")
		}
	}
	if !found {
		t.Fatal("scheduled member not listed as due")
	}

	if err := repo.SetMemberDeleteAfter(group.ID, []uint{drop.ID}, nil); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	due, err = repo.ListMembersDueForDeletion(time.Now(), 10)
	if err != nil {
		t.Fatalf("ListMembersDueForDeletion: %v", err)
	}
	for _, m := range due {
		if m.SceneID == drop.ID {
			t.Fatal("cancelled deletion still listed")
		}
	}

	// duplication_config new columns
	cfg := &DuplicationConfigRecord{FingerprintMode: "dual", DeleteResolvedFiles: true, DeletionDelayHours: 12, BestSelectionCriteria: "duration,resolution"}
	cfgRepo := NewDuplicationConfigRepository(db)
	if err := cfgRepo.Upsert(cfg); err != nil {
		t.Fatalf("Upsert config: %v", err)
	}
	got, err := cfgRepo.Get()
	if err != nil || got == nil || !got.DeleteResolvedFiles || got.DeletionDelayHours != 12 || got.BestSelectionCriteria != "duration,resolution" {
		t.Fatalf("unexpected config: %+v, %v", got, err)
	}
}
