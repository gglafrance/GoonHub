package data

import (
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Integration test of the tag merge against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a
// disposable database, see importTestDB).
func TestTagMergeIntegration(t *testing.T) {
	f := newAliasFixture(t)
	t.Cleanup(func() {
		if err := f.db.Where("name LIKE ?", "%"+f.suffix+"%").Delete(&Tag{}).Error; err != nil {
			t.Errorf("cleanup tags: %v", err)
		}
	})
	repo := NewTagRepository(f.db)

	u1 := &User{Username: f.n("tagmerge-u1"), Password: "x", Role: "user"}
	target := &Tag{Name: f.n("Cumshot"), Color: "#FF4D4D"}
	srcA := &Tag{Name: f.n("cumshot"), Color: "#6B7280"}
	srcB := &Tag{Name: f.n("cumshot") + " ", Color: "#6B7280"}
	other := &Tag{Name: f.n("Anal"), Color: "#6B7280"}
	f.create(t, u1, target, srcA, srcB, other)

	// s1: target + srcA (dedup), s2: srcA + srcB (two sources), s3: other only, s4: srcB but trashed
	s1, s2, s3, s4 := f.scene(t, "t1", nil), f.scene(t, "t2", nil), f.scene(t, "t3", nil), f.scene(t, "t4", nil)
	f.create(t,
		&SceneTag{SceneID: s1.ID, TagID: target.ID}, &SceneTag{SceneID: s1.ID, TagID: srcA.ID},
		&SceneTag{SceneID: s2.ID, TagID: srcA.ID}, &SceneTag{SceneID: s2.ID, TagID: srcB.ID},
		&SceneTag{SceneID: s3.ID, TagID: other.ID},
		&SceneTag{SceneID: s4.ID, TagID: srcB.ID},
	)
	if err := f.db.Delete(s4).Error; err != nil {
		t.Fatal(err)
	}

	// markers: m1 has target (from label) + srcA (manual), m2 has srcB (from label)
	// (labels are server-wide: unique names, created by the marker label trigger)
	labelA, labelB := "tm-a-"+uintStr(target.ID), "tm-b-"+uintStr(target.ID)
	var m1, m2 uint
	f.db.Raw(`INSERT INTO user_scene_markers (user_id, scene_id, timestamp, label) VALUES (?, ?, 10, ?) RETURNING id`, u1.ID, s1.ID, labelA).Scan(&m1)
	f.db.Raw(`INSERT INTO user_scene_markers (user_id, scene_id, timestamp, label) VALUES (?, ?, 20, ?) RETURNING id`, u1.ID, s2.ID, labelB).Scan(&m2)
	if m1 == 0 || m2 == 0 {
		t.Fatal("markers not created")
	}
	var labelIDA, labelIDB uint
	f.db.Raw(`SELECT label_id FROM user_scene_markers WHERE id = ?`, m1).Scan(&labelIDA)
	f.db.Raw(`SELECT label_id FROM user_scene_markers WHERE id = ?`, m2).Scan(&labelIDB)
	if labelIDA == 0 || labelIDB == 0 {
		t.Fatal("marker labels not created")
	}
	t.Cleanup(func() { f.db.Exec(`DELETE FROM marker_labels WHERE id IN (?, ?)`, labelIDA, labelIDB) })
	f.create(t,
		&MarkerTag{MarkerID: m1, TagID: target.ID, IsFromLabel: true}, &MarkerTag{MarkerID: m1, TagID: srcA.ID, IsFromLabel: false},
		&MarkerTag{MarkerID: m2, TagID: srcB.ID, IsFromLabel: true},
		&MarkerLabelTag{LabelID: labelIDA, TagID: target.ID}, &MarkerLabelTag{LabelID: labelIDA, TagID: srcA.ID},
		&MarkerLabelTag{LabelID: labelIDB, TagID: srcB.ID},
	)

	var playlistID uint
	f.db.Raw(`INSERT INTO playlists (user_id, name) VALUES (?, 'tm') RETURNING id`, u1.ID).Scan(&playlistID)
	f.exec(t, `INSERT INTO playlist_tags (playlist_id, tag_id) VALUES (?, ?), (?, ?)`, playlistID, srcA.ID, playlistID, srcB.ID)

	f.exec(t, `INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (?, ?, 'tm', ?::jsonb)`,
		uuid.New(), u1.ID, `{"selected_tags":["`+srcA.Name+`","`+other.Name+`","`+srcB.Name+`"],"query":"x"}`)
	f.exec(t, `INSERT INTO saved_marker_filters (uuid, user_id, name, filters) VALUES (?, ?, 'tm', ?::jsonb)`,
		uuid.New(), u1.ID, `{"marker_tags":["`+srcB.Name+`"],"tags":["`+other.Name+`"]}`)
	f.exec(t, `INSERT INTO user_settings (user_id, homepage_config) VALUES (?, ?::jsonb)
		ON CONFLICT (user_id) DO UPDATE SET homepage_config = EXCLUDED.homepage_config`,
		u1.ID, `{"tabs":[{"id":"home","sections":[{"type":"tag","config":{"tag_id":`+uintStr(srcA.ID)+`}},{"type":"discover","config":{"tags":["`+srcB.Name+`"]}}]}]}`)

	want := TagMergeStats{ScenesRetagged: 2, MarkersUpdated: 2, LabelTagsUpdated: 2, PlaylistsUpdated: 1, ReferencesUpdated: 3, SourcesDeleted: 2}

	// dry run: counts only, nothing changes
	stats, ids, err := repo.MergeTags(*target, []Tag{*srcA, *srcB}, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if *stats != want || ids != nil {
		t.Fatalf("dry run stats = %+v ids = %v, want %+v", *stats, ids, want)
	}
	var n int64
	f.db.Model(&Tag{}).Where("id IN ?", []uint{srcA.ID, srcB.ID}).Count(&n)
	if n != 2 {
		t.Fatalf("dry run deleted the sources")
	}

	stats, ids, err = repo.MergeTags(*target, []Tag{*srcA, *srcB}, false)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if *stats != want {
		t.Fatalf("stats = %+v, want %+v", *stats, want)
	}
	if !equalUints(ids, []uint{s1.ID, s2.ID}) {
		t.Fatalf("scenes to re-index = %v", ids)
	}

	// scene_tags: each scene carries the target once
	for _, s := range []*Scene{s1, s2, s4} {
		var rows []SceneTag
		f.db.Where("scene_id = ?", s.ID).Find(&rows)
		if len(rows) != 1 || rows[0].TagID != target.ID {
			t.Fatalf("scene %s tags = %+v", s.Title, rows)
		}
	}
	tags, _ := repo.GetSceneTags(s3.ID)
	if len(tags) != 1 || tags[0].ID != other.ID {
		t.Fatalf("unrelated scene changed: %+v", tags)
	}

	// marker tags: m1 keeps one target row, no longer "from label" (it was also set by hand)
	var mt []MarkerTag
	f.db.Where("marker_id IN ?", []uint{m1, m2}).Order("marker_id").Find(&mt)
	if len(mt) != 2 || mt[0].MarkerID != m1 || mt[0].TagID != target.ID || mt[0].IsFromLabel ||
		mt[1].MarkerID != m2 || mt[1].TagID != target.ID || !mt[1].IsFromLabel {
		t.Fatalf("marker tags = %+v", mt)
	}
	var lt []MarkerLabelTag
	f.db.Where("label_id IN ?", []uint{labelIDA, labelIDB}).Order("label_id").Find(&lt)
	if len(lt) != 2 || lt[0].LabelID != labelIDA || lt[0].TagID != target.ID || lt[1].LabelID != labelIDB || lt[1].TagID != target.ID {
		t.Fatalf("label tags = %+v", lt)
	}
	var pt []PlaylistTag
	f.db.Where("playlist_id = ?", playlistID).Find(&pt)
	if len(pt) != 1 || pt[0].TagID != target.ID {
		t.Fatalf("playlist tags = %+v", pt)
	}

	// saved filters and homepage
	var ss SavedSearch
	f.db.Where("user_id = ?", u1.ID).First(&ss)
	if !sameStringSlices(ss.Filters.SelectedTags, []string{target.Name, other.Name}) || ss.Filters.Query != "x" {
		t.Fatalf("saved search filters = %+v", ss.Filters)
	}
	var mf SavedMarkerFilter
	f.db.Where("user_id = ?", u1.ID).First(&mf)
	if !sameStringSlices(mf.Filters.MarkerTags, []string{target.Name}) || !sameStringSlices(mf.Filters.Tags, []string{other.Name}) {
		t.Fatalf("saved marker filter = %+v", mf.Filters)
	}
	var homepage string
	f.db.Raw(`SELECT homepage_config::text FROM user_settings WHERE user_id = ?`, u1.ID).Scan(&homepage)
	if !strings.Contains(homepage, `"tag_id": `+uintStr(target.ID)) || strings.Contains(homepage, srcB.Name) || !strings.Contains(homepage, target.Name) {
		t.Fatalf("homepage config = %s", homepage)
	}

	// sources deleted, target and other kept
	f.db.Model(&Tag{}).Where("id IN ?", []uint{srcA.ID, srcB.ID}).Count(&n)
	if n != 0 {
		t.Fatalf("%d source tags left", n)
	}
	f.db.Model(&Tag{}).Where("id IN ?", []uint{target.ID, other.ID}).Count(&n)
	if n != 2 {
		t.Fatalf("target or other tag deleted")
	}

	if _, _, err := repo.MergeTags(*target, []Tag{*target}, false); err == nil {
		t.Fatal("merging a tag into itself must fail")
	}

	// Update (tag color)
	updated := *other
	updated.Color = "#22C55E"
	if err := repo.Update(&updated); err != nil {
		t.Fatalf("update: %v", err)
	}
	stored, _ := repo.GetByID(other.ID)
	if stored.Color != "#22C55E" || stored.Name != other.Name {
		t.Fatalf("updated tag = %+v", stored)
	}
	if err := repo.Update(&Tag{ID: srcA.ID, Name: "x", Color: "#000000"}); err == nil {
		t.Fatal("updating a deleted tag must fail")
	}
}

func uintStr(v uint) string { return strconv.FormatUint(uint64(v), 10) }
