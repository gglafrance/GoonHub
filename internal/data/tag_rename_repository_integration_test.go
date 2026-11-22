package data

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration test of the tag rename against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a
// disposable database, see importTestDB).
func TestTagRenameIntegration(t *testing.T) {
	f := newAliasFixture(t)
	t.Cleanup(func() {
		if err := f.db.Where("name LIKE ?", "%"+f.suffix+"%").Delete(&Tag{}).Error; err != nil {
			t.Errorf("cleanup tags: %v", err)
		}
	})
	repo := NewTagRepository(f.db)

	u1 := &User{Username: f.n("tagrename-u1"), Password: "x", Role: "user"}
	tag := &Tag{Name: f.n("cumshot"), Color: "#6B7280"}
	other := &Tag{Name: f.n("Anal"), Color: "#6B7280"}
	f.create(t, u1, tag, other)
	newName := f.n("Facial")

	// s1, s2 carry the tag, s3 only the other one, s4 the tag but is trashed
	s1, s2, s3, s4 := f.scene(t, "r1", nil), f.scene(t, "r2", nil), f.scene(t, "r3", nil), f.scene(t, "r4", nil)
	f.create(t,
		&SceneTag{SceneID: s1.ID, TagID: tag.ID}, &SceneTag{SceneID: s1.ID, TagID: other.ID},
		&SceneTag{SceneID: s2.ID, TagID: tag.ID},
		&SceneTag{SceneID: s3.ID, TagID: other.ID},
		&SceneTag{SceneID: s4.ID, TagID: tag.ID},
	)
	if err := f.db.Delete(s4).Error; err != nil {
		t.Fatal(err)
	}
	// legacy name arrays: s1 already lists the new name (deduplicated), s3 is untouched
	f.exec(t, `UPDATE scenes SET tags = ? WHERE id = ?`, pq.StringArray{tag.Name, other.Name, newName}, s1.ID)
	f.exec(t, `UPDATE scenes SET tags = ? WHERE id = ?`, pq.StringArray{other.Name}, s3.ID)

	f.exec(t, `INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (?, ?, 'tr', ?::jsonb)`,
		uuid.New(), u1.ID, `{"selected_tags":["`+other.Name+`","`+tag.Name+`"],"query":"x"}`)
	f.exec(t, `INSERT INTO saved_marker_filters (uuid, user_id, name, filters) VALUES (?, ?, 'tr', ?::jsonb)`,
		uuid.New(), u1.ID, `{"marker_tags":["`+tag.Name+`"],"tags":["`+other.Name+`"]}`)
	f.exec(t, `INSERT INTO user_settings (user_id, homepage_config) VALUES (?, ?::jsonb)
		ON CONFLICT (user_id) DO UPDATE SET homepage_config = EXCLUDED.homepage_config`,
		u1.ID, `{"tabs":[{"id":"home","sections":[{"type":"tag","config":{"tag_id":`+uintStr(tag.ID)+`}},`+
			`{"type":"discover","config":{"tags":["`+tag.Name+`"]}},`+
			`{"type":"browse_tags","config":{"source":"tags","items":["`+other.Name+`","`+tag.Name+`"]}}]}]}`)

	renamed := *tag
	renamed.Name = newName
	renamed.Color = "#FF4D4D"
	refs, ids, err := repo.RenameTag(renamed, tag.Name)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if refs != 3 {
		t.Fatalf("references updated = %d, want 3", refs)
	}
	if !equalUints(ids, []uint{s1.ID, s2.ID}) {
		t.Fatalf("scenes to re-index = %v", ids)
	}

	stored, _ := repo.GetByID(tag.ID)
	if stored.Name != newName || stored.Color != "#FF4D4D" {
		t.Fatalf("stored tag = %+v", stored)
	}
	var ss SavedSearch
	f.db.Where("user_id = ?", u1.ID).First(&ss)
	if !sameStringSlices(ss.Filters.SelectedTags, []string{other.Name, newName}) || ss.Filters.Query != "x" {
		t.Fatalf("saved search filters = %+v", ss.Filters)
	}
	var mf SavedMarkerFilter
	f.db.Where("user_id = ?", u1.ID).First(&mf)
	if !sameStringSlices(mf.Filters.MarkerTags, []string{newName}) || !sameStringSlices(mf.Filters.Tags, []string{other.Name}) {
		t.Fatalf("saved marker filter = %+v", mf.Filters)
	}
	var homepage string
	f.db.Raw(`SELECT homepage_config::text FROM user_settings WHERE user_id = ?`, u1.ID).Scan(&homepage)
	if strings.Contains(homepage, tag.Name) || strings.Count(homepage, newName) != 2 ||
		!strings.Contains(homepage, `"tag_id": `+uintStr(tag.ID)) {
		t.Fatalf("homepage config = %s", homepage)
	}
	var legacy pq.StringArray
	if err := f.db.Raw(`SELECT tags FROM scenes WHERE id = ?`, s1.ID).Row().Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if !sameStringSlices(legacy, []string{newName, other.Name}) {
		t.Fatalf("s1 legacy tags = %v", legacy)
	}
	if err := f.db.Raw(`SELECT tags FROM scenes WHERE id = ?`, s3.ID).Row().Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if !sameStringSlices(legacy, []string{other.Name}) {
		t.Fatalf("s3 legacy tags = %v", legacy)
	}

	// a different case of its own name: saved, references follow
	upper := renamed
	upper.Name = strings.ToUpper(newName)
	refs, ids, err = repo.RenameTag(upper, newName)
	if err != nil || refs != 3 || len(ids) != 2 {
		t.Fatalf("case rename: refs %d ids %v err %v", refs, ids, err)
	}
	f.db.Where("user_id = ?", u1.ID).First(&mf)
	if !sameStringSlices(mf.Filters.MarkerTags, []string{upper.Name}) {
		t.Fatalf("saved marker filter after case rename = %+v", mf.Filters)
	}

	// name of another tag: unique violation, nothing changed
	taken := upper
	taken.Name = other.Name
	if _, _, err := repo.RenameTag(taken, upper.Name); !errors.Is(err, ErrTagNameTaken) {
		t.Fatalf("taken name: want ErrTagNameTaken, got %v", err)
	}
	f.db.Where("user_id = ?", u1.ID).First(&mf)
	if !sameStringSlices(mf.Filters.MarkerTags, []string{upper.Name}) {
		t.Fatalf("failed rename changed the filters: %+v", mf.Filters)
	}

	if _, _, err := repo.RenameTag(Tag{ID: 0, Name: f.n("ghost"), Color: "#000000"}, "x"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unknown tag: want not found, got %v", err)
	}
}
