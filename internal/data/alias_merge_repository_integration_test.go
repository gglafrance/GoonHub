package data

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration tests of the alias lookups and the merges, against PostgreSQL
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a disposable database, see importTestDB).

type aliasFixture struct {
	db     *gorm.DB
	suffix string
}

func newAliasFixture(t *testing.T) *aliasFixture {
	t.Helper()
	db := importTestDB(t)
	f := &aliasFixture{db: db, suffix: uuid.NewString()[:8]}
	t.Cleanup(func() {
		like := "%" + f.suffix + "%"
		for _, res := range []*gorm.DB{
			db.Unscoped().Where("stored_path LIKE ?", like).Delete(&Scene{}),
			db.Unscoped().Where("name LIKE ?", like).Delete(&Actor{}),
			db.Exec("UPDATE studios SET parent_id = NULL, network_id = NULL WHERE name LIKE ?", like),
			db.Unscoped().Where("name LIKE ?", like).Delete(&Studio{}),
			db.Where("username LIKE ?", like).Delete(&User{}),
		} {
			if res.Error != nil {
				t.Errorf("cleanup: %v", res.Error)
			}
		}
	})
	return f
}

func (f *aliasFixture) n(name string) string { return name + " " + f.suffix }

func (f *aliasFixture) create(t *testing.T, values ...any) {
	t.Helper()
	for _, v := range values {
		if err := f.db.Create(v).Error; err != nil {
			t.Fatalf("create %T: %v", v, err)
		}
	}
}

func (f *aliasFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := f.db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func (f *aliasFixture) scene(t *testing.T, name string, studio *Studio) *Scene {
	t.Helper()
	s := &Scene{Title: name, StoredPath: "/alias-test/" + f.suffix + "/" + name + ".mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if studio != nil {
		s.StudioID = &studio.ID
		s.Studio = studio.Name
	}
	f.create(t, s)
	return s
}

func TestActorFindByNameOrAliasAndSearch(t *testing.T) {
	f := newAliasFixture(t)
	repo := NewActorRepository(f.db)
	jane := &Actor{Name: f.n("Jane Doe"), Aliases: pq.StringArray{f.n("JD"), f.n("Janie")}}
	other := &Actor{Name: f.n("Someone")}
	gone := &Actor{Name: f.n("Deleted"), Aliases: pq.StringArray{f.n("Ghost")}}
	f.create(t, jane, other, gone)
	if err := f.db.Delete(gone).Error; err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		names     []string
		excludeID uint
		want      []uint
	}{
		{[]string{strings.ToUpper(f.n("jane doe"))}, 0, []uint{jane.ID}},
		{[]string{strings.ToLower(f.n("JD"))}, 0, []uint{jane.ID}},
		{[]string{f.n("JD"), f.n("someone")}, 0, []uint{jane.ID, other.ID}},
		{[]string{f.n("JD")}, jane.ID, nil},
		{[]string{f.n("Ghost")}, 0, nil},           // soft-deleted
		{[]string{f.n("Jan")}, 0, nil},             // exact match only
		{[]string{"JD " + f.suffix + "x"}, 0, nil}, // no prefix match either
	}
	for _, c := range cases {
		got, err := repo.FindByNameOrAlias(c.names, c.excludeID)
		if err != nil {
			t.Fatalf("FindByNameOrAlias(%v): %v", c.names, err)
		}
		ids := make([]uint, len(got))
		for i, a := range got {
			ids[i] = a.ID
		}
		if len(ids) != len(c.want) || (len(ids) > 0 && !equalUints(ids, c.want)) {
			t.Fatalf("FindByNameOrAlias(%v, %d) = %v, want %v", c.names, c.excludeID, ids, c.want)
		}
	}

	// substring search matches aliases too
	found, total, err := repo.Search("janie "+f.suffix, 1, 20, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(found) != 1 || found[0].ID != jane.ID {
		t.Fatalf("Search by alias: total %d, %+v", total, found)
	}
}

func TestStudioFindByNameOrAliasAndSearch(t *testing.T) {
	f := newAliasFixture(t)
	repo := NewStudioRepository(f.db)
	bz := &Studio{Name: f.n("Brazzers"), Aliases: pq.StringArray{f.n("BZ")}}
	plain := &Studio{Name: f.n("Plain")} // nil aliases: stored as '{}'
	f.create(t, bz, plain)

	got, err := repo.FindByNameOrAlias([]string{strings.ToLower(f.n("BZ"))}, 0)
	if err != nil || len(got) != 1 || got[0].ID != bz.ID {
		t.Fatalf("by alias: %v %v", got, err)
	}
	var stored Studio
	if err := f.db.First(&stored, plain.ID).Error; err != nil || stored.Aliases == nil || len(stored.Aliases) != 0 {
		t.Fatalf("nil aliases must be stored as an empty array: %#v %v", stored.Aliases, err)
	}
	found, total, err := repo.Search("bz "+f.suffix, 1, 20, "")
	if err != nil || total != 1 || len(found) != 1 || found[0].ID != bz.ID {
		t.Fatalf("Search by alias: %d %+v %v", total, found, err)
	}
	if byName, err := repo.GetByName(bz.Name); err != nil || byName.ID != bz.ID {
		t.Fatalf("GetByName: %v %v", byName, err)
	}
}

func TestActorMergeIntegration(t *testing.T) {
	f := newAliasFixture(t)
	repo := NewActorRepository(f.db)
	u1 := &User{Username: f.n("merge-u1"), Password: "x", Role: "user"}
	u2 := &User{Username: f.n("merge-u2"), Password: "x", Role: "user"}
	target := &Actor{Name: f.n("Jane Doe"), Aliases: pq.StringArray{f.n("JD")}}
	source := &Actor{Name: f.n("Jane D"), Aliases: pq.StringArray{f.n("Janie")}, ImageURL: "/actor-images/b.jpg"}
	f.create(t, u1, u2, target, source)
	s1, s2, s3 := f.scene(t, "s1", nil), f.scene(t, "s2", nil), f.scene(t, "s3", nil)
	f.create(t,
		&SceneActor{SceneID: s1.ID, ActorID: target.ID}, &SceneActor{SceneID: s1.ID, ActorID: source.ID},
		&SceneActor{SceneID: s2.ID, ActorID: source.ID},
		&SceneActor{SceneID: s3.ID, ActorID: target.ID},
		&UserActorRating{UserID: u1.ID, ActorID: target.ID, Rating: 4}, &UserActorRating{UserID: u1.ID, ActorID: source.ID, Rating: 2},
		&UserActorRating{UserID: u2.ID, ActorID: source.ID, Rating: 5},
		&UserActorLike{UserID: u2.ID, ActorID: source.ID},
	)
	f.exec(t, `INSERT INTO user_settings (user_id, homepage_config) VALUES (?, ?::jsonb)
		ON CONFLICT (user_id) DO UPDATE SET homepage_config = EXCLUDED.homepage_config`,
		u1.ID, `{"sections":[{"type":"actor","config":{"actor_uuid":"`+source.UUID.String()+`"}}]}`)
	f.exec(t, `INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (?, ?, 'merge', ?::jsonb)`,
		uuid.New(), u1.ID, `{"selected_actors":["`+source.Name+`","`+target.Name+`","Other"],"query":"x"}`)
	f.exec(t, `INSERT INTO saved_marker_filters (uuid, user_id, name, filters) VALUES (?, ?, 'merge', ?::jsonb)`,
		uuid.New(), u2.ID, `{"actors":["`+source.Name+`"]}`)

	preview, err := repo.GetMergePreview(target.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := ActorMergeStats{Scenes: 2, SharedScenes: 1, Ratings: 1, Likes: 1}
	if *preview != want {
		t.Fatalf("preview = %+v, want %+v", *preview, want)
	}

	merged := *target
	merged.Aliases = pq.StringArray{f.n("JD"), source.Name, f.n("Janie")}
	merged.ImageURL = source.ImageURL
	stats, sceneIDs, err := repo.Merge(&merged, source)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if *stats != want {
		t.Fatalf("stats = %+v, want %+v", *stats, want)
	}
	if !equalUints(sceneIDs, []uint{s1.ID, s2.ID, s3.ID}) {
		t.Fatalf("scenes to re-index = %v", sceneIDs)
	}

	// scenes: each credits the target once, denormalized names follow
	for _, s := range []*Scene{s1, s2, s3} {
		actors, err := repo.GetSceneActors(s.ID)
		if err != nil || len(actors) != 1 || actors[0].ID != target.ID {
			t.Fatalf("scene %s actors: %+v %v", s.Title, actors, err)
		}
	}
	var s2Stored Scene
	if err := f.db.First(&s2Stored, s2.ID).Error; err != nil || len(s2Stored.Actors) != 1 || s2Stored.Actors[0] != target.Name {
		t.Fatalf("denormalized actors of s2 = %v (%v)", s2Stored.Actors, err)
	}

	// ratings: the target's own one wins, the others move
	var ratings []UserActorRating
	f.db.Where("actor_id IN ?", []uint{target.ID, source.ID}).Order("user_id").Find(&ratings)
	if len(ratings) != 2 || ratings[0].UserID != u1.ID || ratings[0].Rating != 4 || ratings[1].UserID != u2.ID || ratings[1].Rating != 5 || ratings[1].ActorID != target.ID {
		t.Fatalf("ratings after merge: %+v", ratings)
	}
	var likes []UserActorLike
	f.db.Where("actor_id IN ?", []uint{target.ID, source.ID}).Find(&likes)
	if len(likes) != 1 || likes[0].ActorID != target.ID || likes[0].UserID != u2.ID {
		t.Fatalf("likes after merge: %+v", likes)
	}

	// target saved, source soft-deleted
	var storedTarget Actor
	f.db.First(&storedTarget, target.ID)
	if storedTarget.ImageURL != source.ImageURL || len(storedTarget.Aliases) != 3 {
		t.Fatalf("target after merge: %+v", storedTarget)
	}
	var storedSource Actor
	if err := f.db.Unscoped().First(&storedSource, source.ID).Error; err != nil || !storedSource.DeletedAt.Valid {
		t.Fatalf("source must be soft-deleted: %+v %v", storedSource.DeletedAt, err)
	}

	// homepage section and saved filters point to the target
	var homepage string
	f.db.Raw(`SELECT homepage_config::text FROM user_settings WHERE user_id = ?`, u1.ID).Scan(&homepage)
	if strings.Contains(homepage, source.UUID.String()) || !strings.Contains(homepage, target.UUID.String()) {
		t.Fatalf("homepage config not updated: %s", homepage)
	}
	var savedSearch SavedSearch
	f.db.Where("user_id = ?", u1.ID).First(&savedSearch)
	if !sameStringSlices(savedSearch.Filters.SelectedActors, []string{target.Name, "Other"}) || savedSearch.Filters.Query != "x" {
		t.Fatalf("saved search filters: %+v", savedSearch.Filters)
	}
	var markerFilter SavedMarkerFilter
	f.db.Where("user_id = ?", u2.ID).First(&markerFilter)
	if !sameStringSlices(markerFilter.Filters.Actors, []string{target.Name}) {
		t.Fatalf("saved marker filter: %+v", markerFilter.Filters)
	}

	if _, _, err := repo.Merge(&merged, &merged); err == nil {
		t.Fatal("merging an actor into itself must fail")
	}
}

func TestStudioMergeIntegration(t *testing.T) {
	f := newAliasFixture(t)
	repo := NewStudioRepository(f.db)
	u1 := &User{Username: f.n("smerge-u1"), Password: "x", Role: "user"}
	source := &Studio{Name: f.n("Source Studio"), Logo: "/studio-logos/b.png"}
	f.create(t, u1, source)
	target := &Studio{Name: f.n("Target Studio"), ParentID: &source.ID}
	child := &Studio{Name: f.n("Child Studio"), ParentID: &source.ID, NetworkID: &source.ID}
	f.create(t, target, child)
	moved, kept := f.scene(t, "moved", source), f.scene(t, "kept", target)
	f.create(t,
		&UserStudioRating{UserID: u1.ID, StudioID: source.ID, Rating: 3},
		&UserStudioLike{UserID: u1.ID, StudioID: source.ID},
		&UserStudioLike{UserID: u1.ID, StudioID: target.ID},
	)
	f.exec(t, `INSERT INTO saved_searches (uuid, user_id, name, filters) VALUES (?, ?, 'merge', ?::jsonb)`,
		uuid.New(), u1.ID, `{"studio":"`+source.Name+`"}`)

	merged := *target
	merged.Aliases = pq.StringArray{source.Name}
	merged.Logo = source.Logo
	stats, sceneIDs, err := repo.Merge(&merged, source)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if want := (StudioMergeStats{Scenes: 1, ChildStudios: 1, Ratings: 1, Likes: 0}); *stats != want {
		t.Fatalf("stats = %+v, want %+v", *stats, want)
	}
	if !equalUints(sceneIDs, []uint{moved.ID}) {
		t.Fatalf("moved scenes = %v", sceneIDs)
	}

	var s Scene
	f.db.First(&s, moved.ID)
	if s.StudioID == nil || *s.StudioID != target.ID || s.Studio != target.Name {
		t.Fatalf("moved scene: studio_id %v studio %q", s.StudioID, s.Studio)
	}
	f.db.First(&s, kept.ID)
	if s.StudioID == nil || *s.StudioID != target.ID {
		t.Fatal("the target's scene must stay")
	}
	var storedChild, storedTarget Studio
	f.db.First(&storedChild, child.ID)
	if storedChild.ParentID == nil || *storedChild.ParentID != target.ID || storedChild.NetworkID == nil || *storedChild.NetworkID != target.ID {
		t.Fatalf("child studio: parent %v network %v", storedChild.ParentID, storedChild.NetworkID)
	}
	f.db.First(&storedTarget, target.ID)
	if storedTarget.ParentID != nil {
		t.Fatalf("the target was a child of the source: it must not become its own parent, got %v", *storedTarget.ParentID)
	}
	if storedTarget.Logo != source.Logo || len(storedTarget.Aliases) != 1 {
		t.Fatalf("target after merge: %+v", storedTarget)
	}
	var ratings []UserStudioRating
	f.db.Where("user_id = ?", u1.ID).Find(&ratings)
	if len(ratings) != 1 || ratings[0].StudioID != target.ID {
		t.Fatalf("ratings: %+v", ratings)
	}
	var likes int64
	f.db.Model(&UserStudioLike{}).Where("user_id = ?", u1.ID).Count(&likes)
	if likes != 1 {
		t.Fatalf("likes: %d, want the target's one only", likes)
	}
	var savedSearch SavedSearch
	f.db.Where("user_id = ?", u1.ID).First(&savedSearch)
	if savedSearch.Filters.Studio != target.Name {
		t.Fatalf("saved search studio = %q", savedSearch.Filters.Studio)
	}
	var deleted Studio
	if err := f.db.Unscoped().First(&deleted, source.ID).Error; err != nil || !deleted.DeletedAt.Valid {
		t.Fatal("source studio must be soft-deleted")
	}
	if found, err := repo.FindByNameOrAlias([]string{source.Name}, 0); err != nil || len(found) != 1 || found[0].ID != target.ID {
		t.Fatalf("the source's name now resolves to the target: %+v %v", found, err)
	}
}

func equalUints(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameStringSlices(a, b []string) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
