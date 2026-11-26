package data

import (
	"errors"
	"gorm.io/gorm"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Integration tests of the server-wide marker labels (GOONHUB_TEST_IMPORT_POSTGRES_DSN,
// disposable): referential, case-insensitive matching, rename and its references, merge,
// deletion, global default tags and their sync on the markers of every user.
func TestMarkerLabelRepository(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	other := User{Username: f.user.Username + "-b", Password: "x", Role: "user"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { db.Delete(&other) })
	suffix := f.user.Username[len("import-test-"):]
	repo := NewMarkerLabelRepository(db)
	markers := &MarkerRepositoryImpl{DB: db}

	scene := &Scene{Title: "labels", StoredPath: "/import-test/" + suffix + "/labels.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := db.Create(scene).Error; err != nil {
		t.Fatalf("create scene: %v", err)
	}

	name := func(base string) string { return base + " " + suffix }
	var labelIDs []uint
	t.Cleanup(func() {
		db.Where("user_id IN ?", []uint{f.user.ID, other.ID}).Delete(&UserSceneMarker{})
		if len(labelIDs) > 0 {
			db.Where("id IN ?", labelIDs).Delete(&MarkerLabel{})
		}
	})

	kiss := &MarkerLabel{Name: name("Kiss"), Color: "#FF0000"}
	if err := repo.Create(kiss); err != nil {
		t.Fatalf("create: %v", err)
	}
	labelIDs = append(labelIDs, kiss.ID)
	if err := repo.Create(&MarkerLabel{Name: name("KISS"), Color: "#FFFFFF"}); !errors.Is(err, ErrMarkerLabelNameTaken) {
		t.Fatalf("create with another case = %v, want ErrMarkerLabelNameTaken", err)
	}
	found, err := repo.FindByName("  " + name("kiss") + " ")
	if err != nil || found == nil || found.ID != kiss.ID {
		t.Fatalf("find ignoring case = %+v, %v", found, err)
	}
	if missing, err := repo.FindByName(name("nope")); err != nil || missing != nil {
		t.Fatalf("find unknown = %+v, %v", missing, err)
	}
	hug, created, err := repo.Ensure(name("Hug"))
	if err != nil || !created {
		t.Fatalf("ensure new = %+v, %v, %v", hug, created, err)
	}
	labelIDs = append(labelIDs, hug.ID)
	again, created, err := repo.Ensure(name("hug"))
	if err != nil || created || again.ID != hug.ID {
		t.Fatalf("ensure existing = %+v, %v, %v", again, created, err)
	}

	// markers of both users; a marker written with label_id gets the name
	mk := func(user uint, ts int, label *MarkerLabel) *UserSceneMarker {
		t.Helper()
		m := &UserSceneMarker{UserID: user, SceneID: scene.ID, Timestamp: ts, Color: "#FFFFFF"}
		if label != nil {
			m.LabelID = &label.ID
		}
		if err := markers.Create(m); err != nil {
			t.Fatalf("create marker: %v", err)
		}
		got, err := markers.GetByID(m.ID)
		if err != nil {
			t.Fatalf("get marker: %v", err)
		}
		return got
	}
	m1 := mk(f.user.ID, 10, kiss)
	m2 := mk(other.ID, 20, kiss)
	m3 := mk(other.ID, 30, hug)
	if m1.Label != kiss.Name || m3.Label != hug.Name {
		t.Fatalf("label copy = %q / %q", m1.Label, m3.Label)
	}

	// default tags are global: both users' kiss markers get them
	tagA, tagB := f.tags[0], f.tags[1]
	if err := markers.AddMarkerTags(m2.ID, []uint{tagB.ID}); err != nil { // by hand
		t.Fatal(err)
	}
	if err := repo.SetDefaultTags(kiss.ID, []uint{tagA.ID, tagB.ID}); err != nil {
		t.Fatalf("set default tags: %v", err)
	}
	assertTags := func(m *UserSceneMarker, want map[uint]bool) {
		t.Helper()
		tags, err := markers.GetMarkerTags(m.ID)
		if err != nil {
			t.Fatal(err)
		}
		got := map[uint]bool{}
		for _, tg := range tags {
			got[tg.ID] = tg.IsFromLabel
		}
		if len(got) != len(want) {
			t.Fatalf("marker %d tags = %v, want %v", m.ID, got, want)
		}
		for id, fromLabel := range want {
			if v, ok := got[id]; !ok || v != fromLabel {
				t.Fatalf("marker %d tags = %v, want %v", m.ID, got, want)
			}
		}
	}
	assertTags(m1, map[uint]bool{tagA.ID: true, tagB.ID: true})
	assertTags(m2, map[uint]bool{tagA.ID: true, tagB.ID: false})
	if err := repo.SetDefaultTags(kiss.ID, []uint{tagB.ID}); err != nil {
		t.Fatal(err)
	}
	assertTags(m1, map[uint]bool{tagB.ID: true})
	assertTags(m2, map[uint]bool{tagB.ID: false})

	// a new marker gets its label tags; moving it to another label swaps them
	m4 := mk(f.user.ID, 40, kiss)
	if err := markers.ApplyLabelTagsToMarker(m4.ID); err != nil {
		t.Fatal(err)
	}
	assertTags(m4, map[uint]bool{tagB.ID: true})
	m4.LabelID = &hug.ID
	if err := markers.Update(m4); err != nil {
		t.Fatal(err)
	}
	if err := markers.ApplyLabelTagsToMarker(m4.ID); err != nil {
		t.Fatal(err)
	}
	assertTags(m4, map[uint]bool{})
	if got, _ := markers.GetByID(m4.ID); got.Label != hug.Name {
		t.Fatalf("label copy after label_id change = %q", got.Label)
	}

	// filters match labels ignoring case
	groups, total, err := markers.GetLabelGroupsForUser(f.user.ID, 0, 50, MarkerListOptions{Labels: []string{" " + name("KISS")}})
	if err != nil || total != 1 || len(groups) != 1 || groups[0].Label != kiss.Name {
		t.Fatalf("label groups filtered = %+v (%d, %v)", groups, total, err)
	}
	byLabel, total, err := markers.GetMarkersByLabelForUser(f.user.ID, name("kiss"), 0, 50, MarkerListOptions{})
	if err != nil || total != 1 || len(byLabel) != 1 || byLabel[0].ID != m1.ID {
		t.Fatalf("markers by label = %d (%v)", total, err)
	}
	sceneIDs, err := markers.GetSceneIDsByLabels(other.ID, []string{name("hug")})
	if err != nil || len(sceneIDs) != 1 || sceneIDs[0] != scene.ID {
		t.Fatalf("scene ids by label = %v (%v)", sceneIDs, err)
	}

	// suggestions list the referential, by usage of the user's visible markers
	sugg, err := markers.GetLabelSuggestionsForUser(other.ID, 5000)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[uint]int64{}
	for _, s := range sugg {
		counts[s.ID] = s.Count
	}
	if counts[kiss.ID] != 1 || counts[hug.ID] != 1 {
		t.Fatalf("suggestion counts = kiss %d, hug %d", counts[kiss.ID], counts[hug.ID])
	}

	// rename: markers and stored references follow
	mustExec(t, db, `INSERT INTO saved_marker_filters (uuid, user_id, name, filters) VALUES (?, ?, 'ml', ?::jsonb)`,
		uuid.New(), f.user.ID, `{"labels":["`+name("kiss")+`","x"]}`)
	mustExec(t, db, `INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (?, ?, 'ml', ?::jsonb)`,
		uuid.New(), f.user.ID, `{"selected_marker_labels":["`+name("KISS")+`"]}`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM saved_marker_filters WHERE user_id = ?`, f.user.ID)
		db.Exec(`DELETE FROM saved_searches WHERE user_id = ?`, f.user.ID)
	})
	oldName := kiss.Name
	kiss.Name = name("Kissing")
	refs, err := repo.Update(kiss, oldName)
	if err != nil || refs != 2 {
		t.Fatalf("rename = %d refs (%v), want 2", refs, err)
	}
	if got, _ := markers.GetByID(m2.ID); got.Label != kiss.Name {
		t.Fatalf("marker label after rename = %q", got.Label)
	}
	var filters string
	db.Raw(`SELECT filters->>'labels' FROM saved_marker_filters WHERE user_id = ?`, f.user.ID).Scan(&filters)
	if filters != `["`+kiss.Name+`", "x"]` {
		t.Fatalf("saved marker filter labels = %s", filters)
	}
	if _, err := repo.Update(&MarkerLabel{ID: hug.ID, Name: name("KISSING"), Color: "#FFFFFF"}, hug.Name); !errors.Is(err, ErrMarkerLabelNameTaken) {
		t.Fatalf("rename onto another label = %v", err)
	}

	// merge hug into kiss (dry run first)
	stats, err := repo.Merge(*kiss, []MarkerLabel{*hug}, true)
	if err != nil || stats.MarkersMoved != 2 || stats.SourcesDeleted != 1 {
		t.Fatalf("dry run = %+v (%v)", stats, err)
	}
	if still, _ := repo.GetByID(hug.ID); still == nil {
		t.Fatal("dry run deleted the source")
	}
	stats, err = repo.Merge(*kiss, []MarkerLabel{*hug}, false)
	if err != nil || stats.MarkersMoved != 2 {
		t.Fatalf("merge = %+v (%v)", stats, err)
	}
	moved, _ := markers.GetByID(m3.ID)
	if moved.LabelID == nil || *moved.LabelID != kiss.ID || moved.Label != kiss.Name {
		t.Fatalf("moved marker = %+v", moved)
	}
	assertTags(moved, map[uint]bool{tagB.ID: true})

	// usage, then deletion: markers stay, without label nor label tags
	usage, err := repo.ListWithUsage()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range usage {
		if u.ID == kiss.ID && (u.MarkerCount != 4 || u.UserCount != 2 || len(u.DefaultTags) != 1) {
			t.Fatalf("usage = %+v", u)
		}
	}
	cleared, err := repo.Delete([]uint{kiss.ID})
	if err != nil || cleared != 4 {
		t.Fatalf("delete = %d (%v)", cleared, err)
	}
	gone, _ := markers.GetByID(m1.ID)
	if gone.LabelID != nil || gone.Label != "" {
		t.Fatalf("marker after label deletion = %+v", gone)
	}
	assertTags(gone, map[uint]bool{})
	assertTags(m2, map[uint]bool{tagB.ID: false})

	// a legacy text write resolves to the label (created when missing)
	legacy := &UserSceneMarker{UserID: f.user.ID, SceneID: scene.ID, Timestamp: 50, Label: name("Legacy"), Color: "#FFFFFF"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	legacyLabel, err := repo.FindByName(name("legacy"))
	if err != nil || legacyLabel == nil {
		t.Fatalf("legacy label not created (%v)", err)
	}
	labelIDs = append(labelIDs, legacyLabel.ID)
	got, _ := markers.GetByID(legacy.ID)
	if got.LabelID == nil || *got.LabelID != legacyLabel.ID {
		t.Fatalf("legacy marker = %+v", got)
	}
	if left, _ := repo.GetByIDs([]uint{kiss.ID, hug.ID}); len(left) != 0 {
		t.Fatalf("labels left = %+v", left)
	}
}

func TestRenameHomepageMarkerLabelRefs(t *testing.T) {
	cfg := map[string]any{
		"tabs": []any{map[string]any{"sections": []any{
			map[string]any{"type": "moments", "config": map[string]any{"labels": []any{"kiss", "Hug", "Kiss "}}},
			map[string]any{"type": "recent_markers", "config": map[string]any{"labels": "Kiss,Other"}},
			map[string]any{"type": "browse_tags", "config": map[string]any{"items": []any{"KISS"}}},
			map[string]any{"type": "browse_tags", "config": map[string]any{"source": "tags", "items": []any{"kiss"}}},
		}}},
	}
	if !RenameHomepageMarkerLabelRefs(cfg, map[string]bool{"kiss": true}, "Kissing") {
		t.Fatal("expected a change")
	}
	sections := cfg["tabs"].([]any)[0].(map[string]any)["sections"].([]any)
	conf := func(i int) map[string]any { return sections[i].(map[string]any)["config"].(map[string]any) }
	if got := conf(0)["labels"].([]any); len(got) != 2 || got[0] != "Kissing" || got[1] != "Hug" {
		t.Fatalf("moments labels = %v", got)
	}
	if got := conf(1)["labels"]; got != "Kissing,Other" {
		t.Fatalf("recent markers labels = %v", got)
	}
	if got := conf(2)["items"].([]any); got[0] != "Kissing" {
		t.Fatalf("browse labels items = %v", got)
	}
	if got := conf(3)["items"].([]any); got[0] != "kiss" {
		t.Fatalf("browse tags items changed: %v", got)
	}
	if RenameHomepageMarkerLabelRefs(cfg, map[string]bool{"absent": true}, "x") {
		t.Fatal("no change expected")
	}
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
