package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/lib/pq"
)

// Integration test of ListSceneGroupMembers (similar scenes of the watch page). Runs on the
// disposable import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestDuplicateGroupRepositoryListSceneGroupMembers(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewDuplicateGroupRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	newScene := func(name string, duration int) *Scene {
		s := &Scene{Title: name, Duration: duration, Width: 1920, Height: 1080, Size: int64(duration) * 1000,
			ThumbnailPath: name + ".jpg", StoredPath: fmt.Sprintf("/import-test/%s/%s.mp4", suffix, name),
			Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	newGroup := func(status string, members ...*DuplicateGroupMember) *DuplicateGroup {
		g := &DuplicateGroup{Status: status, SceneCount: len(members)}
		if err := repo.Create(g); err != nil {
			t.Fatalf("create group: %v", err)
		}
		t.Cleanup(func() { db.Delete(&DuplicateGroup{}, g.ID) })
		for _, m := range members {
			m.GroupID = g.ID
			if m.MatchType == "" {
				m.MatchType = "audio"
			}
			if err := repo.AddMember(m); err != nil {
				t.Fatalf("add member: %v", err)
			}
		}
		return g
	}
	f64 := func(v float64) *float64 { return &v }

	cur, clip, trashed, deleted := newScene("sim-current", 600), newScene("sim-clip", 90), newScene("sim-trashed", 600), newScene("sim-deleted", 600)
	otherA, otherB, alone := newScene("sim-other-a", 300), newScene("sim-other-b", 300), newScene("sim-alone", 300)

	group := newGroup("dismissed",
		&DuplicateGroupMember{SceneID: cur.ID, ConfidenceScore: 0.5, Relation: DuplicateRelationUnknown},
		&DuplicateGroupMember{SceneID: clip.ID, ConfidenceScore: 0.9, MatchType: "visual", Relation: DuplicateRelationContained,
			MatchStart: f64(0), MatchEnd: f64(90), RefSceneID: &cur.ID, RefStart: f64(70), RefEnd: f64(160)},
		&DuplicateGroupMember{SceneID: trashed.ID, ConfidenceScore: 0.7, Relation: DuplicateRelationIdentical, RefSceneID: &cur.ID},
		&DuplicateGroupMember{SceneID: deleted.ID, ConfidenceScore: 0.6, Relation: DuplicateRelationIdentical},
	)
	newGroup("unresolved",
		&DuplicateGroupMember{SceneID: otherA.ID, Relation: DuplicateRelationIdentical},
		&DuplicateGroupMember{SceneID: otherB.ID, Relation: DuplicateRelationIdentical},
	)
	now := time.Now()
	if err := db.Model(&Scene{}).Where("id = ?", trashed.ID).Update("trashed_at", now).Error; err != nil {
		t.Fatalf("trash scene: %v", err)
	}
	if err := db.Delete(&Scene{}, deleted.ID).Error; err != nil {
		t.Fatalf("soft delete scene: %v", err)
	}

	sceneIDs := func(rows []DuplicateMemberScene) []uint {
		ids := make([]uint, len(rows))
		for i, r := range rows {
			ids[i] = r.SceneID
		}
		return ids
	}

	// Not trashed only, by confidence: clip (0.9) then the current scene (0.5)
	rows, err := repo.ListSceneGroupMembers(cur.ID, false)
	if err != nil {
		t.Fatalf("ListSceneGroupMembers: %v", err)
	}
	if got := sceneIDs(rows); fmt.Sprint(got) != fmt.Sprint([]uint{clip.ID, cur.ID}) {
		t.Fatalf("members without trash: got %v, want [%d %d]", got, clip.ID, cur.ID)
	}
	c := rows[0]
	if c.GroupID != group.ID || c.GroupStatus != "dismissed" || c.Title != "sim-clip" || c.Duration != 90 ||
		c.Width != 1920 || c.Height != 1080 || c.Size != 90000 || c.ThumbnailPath != "sim-clip.jpg" ||
		c.MatchType != "visual" || c.ConfidenceScore != 0.9 || c.Relation != DuplicateRelationContained ||
		c.RefSceneID == nil || *c.RefSceneID != cur.ID || c.RefStart == nil || *c.RefStart != 70 ||
		c.MatchEnd == nil || *c.MatchEnd != 90 || c.TrashedAt != nil {
		t.Fatalf("clip row not scanned as stored: %+v", c)
	}

	// With trash: the trashed scene comes in, flagged; the deleted one never does
	rows, err = repo.ListSceneGroupMembers(cur.ID, true)
	if err != nil {
		t.Fatalf("ListSceneGroupMembers: %v", err)
	}
	if got := sceneIDs(rows); fmt.Sprint(got) != fmt.Sprint([]uint{clip.ID, trashed.ID, cur.ID}) {
		t.Fatalf("members with trash: got %v", got)
	}
	if rows[1].TrashedAt == nil {
		t.Fatal("trashed scene not flagged")
	}

	// A trashed current scene is still listed (it is the one asked for)
	rows, err = repo.ListSceneGroupMembers(trashed.ID, false)
	if err != nil {
		t.Fatalf("ListSceneGroupMembers: %v", err)
	}
	if got := sceneIDs(rows); fmt.Sprint(got) != fmt.Sprint([]uint{clip.ID, trashed.ID, cur.ID}) {
		t.Fatalf("members of a trashed scene: got %v", got)
	}

	// A scene in no group has no members
	rows, err = repo.ListSceneGroupMembers(alone.ID, true)
	if err != nil {
		t.Fatalf("ListSceneGroupMembers: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("scene in no group: got %v", sceneIDs(rows))
	}
}
