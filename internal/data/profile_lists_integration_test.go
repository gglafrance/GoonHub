package data

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Integration test of the profile lists SQL (ratings histogram, rated, liked and jizz pages)
// against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a disposable database).
func TestProfileListsIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	owner := User{Username: "plist-owner-" + suffix, Password: "x", Role: "user"}
	other := User{Username: "plist-other-" + suffix, Password: "x", Role: "user"}
	for _, u := range []*User{&owner, &other} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	mkScene := func(name string) *Scene {
		s := &Scene{Title: name + "-" + suffix, StoredPath: "/plist-test/" + suffix + "/" + name + ".mp4", Duration: 60,
			Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	a, b, c, d, trashed := mkScene("a"), mkScene("b"), mkScene("c"), mkScene("d"), mkScene("trashed")
	if err := db.Model(trashed).Update("trashed_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM scenes WHERE stored_path LIKE ?", "/plist-test/"+suffix+"/%")
		db.Where("username LIKE ?", "plist-%-"+suffix).Delete(&User{})
	})
	exec := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	// Ratings: a 4.5 (oldest), b 4.5, c 3.7 (imported, bucket 3.5), d 1, trashed 5 (skipped),
	// and another user's rating (never counted)
	for i, r := range []struct {
		scene  uint
		rating float64
	}{{a.ID, 4.5}, {b.ID, 4.5}, {c.ID, 3.7}, {d.ID, 1}, {trashed.ID, 5}} {
		at := base.Add(time.Duration(i) * time.Hour)
		exec(`INSERT INTO user_scene_ratings (user_id, scene_id, rating, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, owner.ID, r.scene, r.rating, at, at)
	}
	exec(`INSERT INTO user_scene_ratings (user_id, scene_id, rating) VALUES (?, ?, 2)`, other.ID, a.ID)

	repo := NewProfileRepository(db)
	sum, err := repo.RatingSummary(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[float64]int64{}
	for _, bk := range sum.Buckets {
		got[bk.Rating] = bk.Count
	}
	if len(sum.Buckets) != 3 || got[4.5] != 2 || got[3.5] != 1 || got[1] != 1 {
		t.Fatalf("buckets: %+v", sum.Buckets)
	}
	if sum.Average == nil || *sum.Average < 3.42 || *sum.Average > 3.43 { // (4.5+4.5+3.7+1)/4 = 3.425
		t.Fatalf("average: %v", sum.Average)
	}
	if empty, err := repo.RatingSummary(ctx, 999999); err != nil || len(empty.Buckets) != 0 || empty.Average != nil {
		t.Fatalf("no ratings: %+v %v", empty, err)
	}

	ids := func(refs []ProfileSceneRef) []uint {
		out := make([]uint, len(refs))
		for i, r := range refs {
			out[i] = r.SceneID
		}
		return out
	}
	same := func(name string, got []uint, want ...uint) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %v, want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: %v, want %v", name, got, want)
			}
		}
	}

	refs, total, err := repo.RatedScenes(ctx, owner.ID, ProfileListQuery{Page: 1, Limit: 10})
	if err != nil || total != 4 {
		t.Fatalf("rated: %d %v", total, err)
	}
	// rating desc, then most recently rated
	same("rating_desc", ids(refs), b.ID, a.ID, c.ID, d.ID)
	if refs[0].Rating == nil || *refs[0].Rating != 4.5 || refs[0].At == nil {
		t.Fatalf("rated figures: %+v", refs[0])
	}
	refs, _, _ = repo.RatedScenes(ctx, owner.ID, ProfileListQuery{Sort: ProfileSortRatedAsc, Page: 1, Limit: 10})
	same("rated_asc", ids(refs), a.ID, b.ID, c.ID, d.ID)
	refs, _, _ = repo.RatedScenes(ctx, owner.ID, ProfileListQuery{Sort: ProfileSortRatingAsc, Page: 1, Limit: 10})
	same("rating_asc", ids(refs), d.ID, c.ID, b.ID, a.ID)
	// pagination
	refs, total, _ = repo.RatedScenes(ctx, owner.ID, ProfileListQuery{Sort: ProfileSortRatedDesc, Page: 2, Limit: 3})
	same("page 2", ids(refs), a.ID)
	if total != 4 {
		t.Fatalf("page total: %d", total)
	}
	// one bucket: 3.5 holds the imported 3.7
	r35 := 3.5
	refs, total, _ = repo.RatedScenes(ctx, owner.ID, ProfileListQuery{Rating: &r35, Page: 1, Limit: 10})
	same("bucket 3.5", ids(refs), c.ID)
	if total != 1 || *refs[0].Rating != 3.7 {
		t.Fatalf("bucket 3.5: %d %+v", total, refs[0])
	}
	r5 := 5.0
	if refs, total, _ = repo.RatedScenes(ctx, owner.ID, ProfileListQuery{Rating: &r5, Page: 1, Limit: 10}); total != 0 || len(refs) != 0 {
		t.Fatalf("trashed scene in bucket 5: %d", total)
	}

	// Likes: a then c then trashed
	for i, s := range []uint{a.ID, c.ID, trashed.ID} {
		exec(`INSERT INTO user_scene_likes (user_id, scene_id, created_at) VALUES (?, ?, ?)`, owner.ID, s, base.Add(time.Duration(i)*time.Hour))
	}
	refs, total, err = repo.LikedScenesPage(ctx, owner.ID, ProfileListQuery{Page: 1, Limit: 10})
	if err != nil || total != 2 {
		t.Fatalf("liked: %d %v", total, err)
	}
	same("liked_desc", ids(refs), c.ID, a.ID)
	refs, _, _ = repo.LikedScenesPage(ctx, owner.ID, ProfileListQuery{Sort: ProfileSortLikedAsc, Page: 1, Limit: 1})
	same("liked_asc", ids(refs), a.ID)

	// Jizz: a 3 (last event yesterday), b 1 (no event: the count date), c 0 (skipped), trashed 2
	exec(`INSERT INTO user_scene_jizzed (user_id, scene_id, count, updated_at) VALUES (?, ?, 3, ?), (?, ?, 1, ?), (?, ?, 0, ?), (?, ?, 2, ?)`,
		owner.ID, a.ID, base, owner.ID, b.ID, base.Add(48*time.Hour), owner.ID, c.ID, base, owner.ID, trashed.ID, base)
	exec(`INSERT INTO user_scene_jizz_events (user_id, scene_id, jizzed_at) VALUES (?, ?, ?), (?, ?, ?)`,
		owner.ID, a.ID, base.Add(24*time.Hour), owner.ID, a.ID, base.Add(72*time.Hour))
	refs, total, err = repo.JizzScenes(ctx, owner.ID, ProfileListQuery{Page: 1, Limit: 10})
	if err != nil || total != 2 {
		t.Fatalf("jizz: %d %v", total, err)
	}
	same("recent", ids(refs), a.ID, b.ID)
	if refs[0].Count != 3 || refs[0].At == nil || !refs[0].At.Equal(base.Add(72*time.Hour)) {
		t.Fatalf("jizz figures: %+v", refs[0])
	}
	if refs[1].At == nil || !refs[1].At.Equal(base.Add(48*time.Hour)) {
		t.Fatalf("jizz date without events: %+v", refs[1])
	}
	exec(`UPDATE user_scene_jizzed SET count = 9 WHERE user_id = ? AND scene_id = ?`, owner.ID, b.ID)
	refs, _, _ = repo.JizzScenes(ctx, owner.ID, ProfileListQuery{Sort: ProfileSortJizzCount, Page: 1, Limit: 10})
	same("count_desc", ids(refs), b.ID, a.ID)
	js, err := repo.JizzSummary(ctx, owner.ID)
	if err != nil || js.Total != 12 || js.Scenes != 2 {
		t.Fatalf("jizz summary: %+v %v", js, err)
	}
}

// Integration test of the tabs column and the custom lists table (migration 103).
func TestProfileTabsAndCustomListsIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	u := User{Username: "ptabs-" + suffix, Password: "x", Role: "user"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Where("id = ?", u.ID).Delete(&User{}) })
	repo := NewProfileRepository(db)

	tabs := ProfileTabs{
		{ID: "profile", Kind: ProfileTabProfile, Visible: true, Sections: []ProfileSection{{ID: "s", Template: "2-1", Columns: [][]ProfileBlockConfig{
			{{ID: "st", Type: "stats", Visible: true, Config: map[string]any{"period": "year"}}}, {},
		}}}},
		{ID: "d", Kind: ProfileTabCustom, Title: "Discover", Slug: "discover", Icon: "heroicons:fire", Visible: true},
	}
	if err := repo.SaveLayout(ctx, u.ID, ProfileLayout{{Type: "stats", Visible: true}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveTabs(ctx, u.ID, tabs); err != nil {
		t.Fatal(err)
	}
	p, err := repo.GetProfile(ctx, u.ID)
	if err != nil || len(p.Tabs) != 2 || p.Tabs[1].Slug != "discover" || p.Tabs[0].Sections[0].Columns[0][0].Config["period"] != "year" ||
		len(p.Tabs[0].Sections[0].Columns[1]) != 0 || p.Layout == nil {
		t.Fatalf("tabs roundtrip: %+v %v", p, err)
	}
	// Reset: tabs and the older layout cleared
	if err := repo.SaveTabs(ctx, u.ID, nil); err != nil {
		t.Fatal(err)
	}
	if p, _ = repo.GetProfile(ctx, u.ID); p.Tabs != nil || p.Layout != nil {
		t.Fatalf("reset: %+v %+v", p.Tabs, p.Layout)
	}

	min := 2
	liked := true
	a := &ProfileList{UserID: u.ID, Title: "Top O", Audience: "public", Filters: Filters{Sort: "jizz_count_desc", MinJizzCount: &min}}
	b := &ProfileList{UserID: u.ID, Title: "Liked", Audience: "friends", Figures: "auto", Filters: Filters{Liked: &liked}}
	for _, l := range []*ProfileList{a, b} {
		if err := repo.CreateList(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := repo.CountLists(ctx, u.ID); err != nil || n != 2 {
		t.Fatalf("count: %d %v", n, err)
	}
	ssid := uuid.NewString()
	b.Title, b.SavedSearchUUID, b.Figures = "Liked lately", &ssid, "rating"
	if err := repo.UpdateList(ctx, b); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetList(ctx, u.ID, b.ID)
	if err != nil || got.Title != "Liked lately" || got.SavedSearchUUID == nil || *got.SavedSearchUUID != ssid || got.Filters.Liked == nil ||
		got.Figures != "rating" {
		t.Fatalf("updated: %+v %v", got, err)
	}
	if _, err := repo.GetList(ctx, u.ID+1000000, b.ID); err == nil {
		t.Fatal("someone else read the list")
	}
	lists, err := repo.ListsByIDs(ctx, u.ID, []uint{a.ID, 999999})
	// a list created without figures gets the column default (auto)
	if err != nil || len(lists) != 1 || lists[0].Filters.MinJizzCount == nil || *lists[0].Filters.MinJizzCount != 2 || lists[0].Figures != "auto" {
		t.Fatalf("by ids: %+v %v", lists, err)
	}
	if all, _ := repo.Lists(ctx, u.ID); len(all) != 2 || all[0].ID != a.ID {
		t.Fatalf("lists: %+v", all)
	}
	if ok, err := repo.DeleteList(ctx, u.ID+1000000, a.ID); ok || err != nil {
		t.Fatalf("someone else deleted the list: %v %v", ok, err)
	}
	if ok, err := repo.DeleteList(ctx, u.ID, a.ID); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}
}

func TestInteractionSortKeysIntegration(t *testing.T) {
	db := importTestDB(t)
	suffix := uuid.NewString()[:8]
	u := User{Username: "psort-" + suffix, Password: "x", Role: "user"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	s := &Scene{Title: "s-" + suffix, StoredPath: "/psort/" + suffix + ".mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM scenes WHERE id = ?", s.ID)
		db.Where("id = ?", u.ID).Delete(&User{})
	})
	at := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	db.Exec(`INSERT INTO user_scene_likes (user_id, scene_id, created_at) VALUES (?, ?, ?)`, u.ID, s.ID, at)
	db.Exec(`INSERT INTO user_scene_watches (user_id, scene_id, watched_at) VALUES (?, ?, ?), (?, ?, ?)`, u.ID, s.ID, at, u.ID, s.ID, at.Add(time.Hour))
	repo := NewInteractionRepository(db)
	likes, err := repo.GetLikeTimes(u.ID, []uint{s.ID, 999999})
	if err != nil || len(likes) != 1 || !likes[s.ID].Equal(at) {
		t.Fatalf("like times: %v %v", likes, err)
	}
	stats, err := repo.GetWatchStats(u.ID, []uint{s.ID})
	if err != nil || stats[s.ID].Count != 2 || !stats[s.ID].Last.Equal(at.Add(time.Hour)) {
		t.Fatalf("watch stats: %v %v", stats, err)
	}
}
