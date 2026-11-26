package data

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Integration test of the profile SQL against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a
// disposable database, see importTestDB).
func TestProfileRepositoryIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	mkUser := func(name string, share bool) User {
		u := User{Username: "prof-" + name + "-" + suffix, Password: "x", Role: "user", ShareMarkers: share}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u
	}
	owner, sharer, hider := mkUser("owner", false), mkUser("sharer", true), mkUser("hider", false)
	mkScene := func(name string) *Scene {
		s := &Scene{Title: name + "-" + suffix, StoredPath: "/prof-test/" + suffix + "/" + name + ".mp4", Duration: 60,
			ThumbnailPath: "t.webp", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
		return s
	}
	a, b, c, trashed := mkScene("a"), mkScene("b"), mkScene("c"), mkScene("trashed")
	now := time.Now()
	if err := db.Model(trashed).Update("trashed_at", now).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM scenes WHERE stored_path LIKE ?", "/prof-test/"+suffix+"/%")
		db.Where("username LIKE ?", "%-"+suffix).Delete(&User{})
	})
	repo := NewProfileRepository(db)
	social := NewSocialRepository(db)

	// Profile: defaults, then fields, avatar, banner, layout
	p, err := repo.GetProfile(ctx, owner.ID)
	if err != nil || p.BannerType != ProfileBannerNone || p.Layout != nil || p.AvatarVersion != 0 {
		t.Fatalf("default profile: %+v %v", p, err)
	}
	if err := repo.SaveProfile(ctx, owner.ID, "The Owner", "hello\nworld"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAvatarVersion(ctx, owner.ID, 1234); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetBanner(ctx, owner.ID, ProfileBannerScene, &a.ID, 0); err != nil {
		t.Fatal(err)
	}
	layout := ProfileLayout{{Type: "stats", Visible: true, Config: map[string]any{"period": "year"}}, {Type: "friends", LockedPlaceholder: true, Config: map[string]any{}}}
	if err := repo.SaveLayout(ctx, owner.ID, layout); err != nil {
		t.Fatal(err)
	}
	p, err = repo.GetProfile(ctx, owner.ID)
	if err != nil || p.DisplayName != "The Owner" || p.Bio != "hello\nworld" || p.AvatarVersion != 1234 ||
		p.BannerType != ProfileBannerScene || p.BannerSceneID == nil || *p.BannerSceneID != a.ID {
		t.Fatalf("saved profile: %+v %v", p, err)
	}
	if len(p.Layout) != 2 || p.Layout[0].Type != "stats" || p.Layout[0].Config["period"] != "year" || !p.Layout[1].LockedPlaceholder {
		t.Fatalf("layout roundtrip: %+v", p.Layout)
	}
	if err := repo.SaveLayout(ctx, owner.ID, nil); err != nil {
		t.Fatal(err)
	}
	if p, _ = repo.GetProfile(ctx, owner.ID); p.Layout != nil {
		t.Fatalf("reset layout: %+v", p.Layout)
	}
	if err := db.Exec(`UPDATE user_profiles SET bio = repeat('x', 5001) WHERE user_id = ?`, owner.ID).Error; err == nil {
		t.Fatal("bio length check not enforced")
	}

	// The social cards carry the display name and the avatar version
	card, err := social.GetUser(ctx, owner.ID)
	if err != nil || card.DisplayName != "The Owner" || card.AvatarVersion != 1234 {
		t.Fatalf("social card: %+v %v", card, err)
	}
	if card, err = social.GetUser(ctx, sharer.ID); err != nil || card.DisplayName != "" || card.AvatarVersion != 0 {
		t.Fatalf("card without profile: %+v %v", card, err)
	}

	// Showcases keep their order; trashed scenes are skipped when read
	if err := repo.SetFavoriteScenes(ctx, owner.ID, []uint{c.ID, trashed.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	ids, err := repo.FavoriteSceneIDs(ctx, owner.ID)
	if err != nil || len(ids) != 3 || ids[0] != c.ID || ids[2] != a.ID {
		t.Fatalf("favorite ids: %v %v", ids, err)
	}
	scenes, err := repo.ScenesByIDs(ctx, ids)
	if err != nil || len(scenes) != 2 || scenes[0].ID != c.ID || scenes[1].ID != a.ID {
		t.Fatalf("scenes by ids: %v %v", scenes, err)
	}
	live, err := repo.LiveSceneIDs(ctx, []uint{a.ID, trashed.ID, 0})
	if err != nil || !live[a.ID] || live[trashed.ID] || len(live) != 1 {
		t.Fatalf("live ids: %v %v", live, err)
	}
	if err := repo.SetFavoriteScenes(ctx, owner.ID, []uint{b.ID}); err != nil {
		t.Fatal(err)
	}
	if ids, _ = repo.FavoriteSceneIDs(ctx, owner.ID); len(ids) != 1 || ids[0] != b.ID {
		t.Fatalf("replaced showcase: %v", ids)
	}

	// Markers: own, shared by another user, hidden (not shared), on a trashed scene
	mkMarker := func(userID, sceneID uint, label string, at time.Time) UserSceneMarker {
		m := UserSceneMarker{UserID: userID, SceneID: sceneID, Timestamp: 10, Label: label, Color: "#FFFFFF", CreatedAt: at, UpdatedAt: at}
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
		return m
	}
	own1 := mkMarker(owner.ID, a.ID, "kiss", now.Add(-2*time.Hour))
	own2 := mkMarker(owner.ID, b.ID, "dance", now.Add(-time.Hour))
	shared := mkMarker(sharer.ID, a.ID, "shared", now)
	hidden := mkMarker(hider.ID, a.ID, "hidden", now)
	onTrash := mkMarker(owner.ID, trashed.ID, "trash", now)
	visible, err := repo.VisibleMarkers(ctx, owner.ID, []uint{shared.ID, hidden.ID, own1.ID, onTrash.ID})
	if err != nil || len(visible) != 2 || visible[0].ID != shared.ID || visible[1].ID != own1.ID || visible[1].SceneTitle != a.Title {
		t.Fatalf("visible markers: %+v %v", visible, err)
	}
	if err := repo.SetFavoriteMarkers(ctx, owner.ID, []uint{own2.ID, shared.ID}); err != nil {
		t.Fatal(err)
	}
	if mids, _ := repo.FavoriteMarkerIDs(ctx, owner.ID); len(mids) != 2 || mids[0] != own2.ID {
		t.Fatalf("favorite markers: %v", mids)
	}
	recent, err := repo.RecentMarkers(ctx, owner.ID, 10)
	if err != nil || len(recent) != 2 || recent[0].ID != own2.ID || recent[1].ID != own1.ID {
		t.Fatalf("recent markers (own only, live scenes): %+v %v", recent, err)
	}
	cands, total, err := repo.MarkerCandidates(ctx, owner.ID, "", 1, 10)
	if err != nil || total != 3 || len(cands) != 3 {
		t.Fatalf("marker candidates: %d %+v %v", total, cands, err)
	}
	if cands, total, err = repo.MarkerCandidates(ctx, owner.ID, "dan", 1, 10); err != nil || total != 1 || cands[0].ID != own2.ID {
		t.Fatalf("marker candidates search: %d %+v %v", total, cands, err)
	}
	if _, total, err = repo.MarkerCandidates(ctx, owner.ID, "100%_", 1, 10); err != nil || total != 0 {
		t.Fatalf("marker candidates wildcards: %d %v", total, err)
	}

	// Watches: a x3 (two recent, one old), b x1, trashed x1
	watch := func(sceneID uint, at time.Time, secs int) {
		if err := db.Exec("INSERT INTO user_scene_watches (user_id, scene_id, watched_at, watch_duration) VALUES (?, ?, ?, ?)", owner.ID, sceneID, at, secs).Error; err != nil {
			t.Fatal(err)
		}
	}
	watch(a.ID, now.Add(-3*time.Hour), 100)
	watch(a.ID, now.Add(-2*time.Hour), 100)
	watch(a.ID, now.AddDate(0, -2, 0), 100)
	watch(b.ID, now.Add(-time.Hour), 50)
	watch(trashed.ID, now, 10)
	rw, err := repo.RecentWatches(ctx, owner.ID, 10)
	if err != nil || len(rw) != 2 || rw[0].SceneID != b.ID || rw[1].SceneID != a.ID || rw[1].Count != 3 {
		t.Fatalf("recent watches: %+v %v", rw, err)
	}
	mw, err := repo.MostWatched(ctx, owner.ID, now.AddDate(0, 0, -30), 10)
	if err != nil || len(mw) != 2 || mw[0].SceneID != a.ID || mw[0].Count != 2 {
		t.Fatalf("most watched: %+v %v", mw, err)
	}
	if mw, _ = repo.MostWatched(ctx, owner.ID, time.Time{}, 10); mw[0].Count != 3 {
		t.Fatalf("most watched all time: %+v", mw)
	}

	// Likes, o, ratings, tags
	if err := db.Exec("INSERT INTO user_scene_likes (user_id, scene_id) VALUES (?, ?), (?, ?), (?, ?)", owner.ID, a.ID, owner.ID, b.ID, owner.ID, trashed.ID).Error; err != nil {
		t.Fatal(err)
	}
	liked, likedTotal, err := repo.LikedScenes(ctx, owner.ID, 1)
	if err != nil || likedTotal != 2 || len(liked) != 1 || liked[0].At == nil {
		t.Fatalf("liked: %+v %d %v", liked, likedTotal, err)
	}
	if err := db.Exec("INSERT INTO user_scene_jizz_events (user_id, scene_id, jizzed_at) VALUES (?, ?, ?), (?, ?, ?)", owner.ID, a.ID, now.Add(-time.Hour), owner.ID, a.ID, now.AddDate(-2, 0, 0)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_scene_jizzed (user_id, scene_id, count) VALUES (?, ?, 5)", owner.ID, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_scene_ratings (user_id, scene_id, rating) VALUES (?, ?, 4.0)", owner.ID, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	st, err := repo.Stats(ctx, owner.ID, now.AddDate(0, 0, -30))
	if err != nil || st.Views != 4 || st.WatchTime != 260 || st.ScenesWatched != 2 || st.Jizz != 1 || st.Liked != 2 || st.Markers != 2 || st.Rated != 1 || st.AvgRating == nil {
		t.Fatalf("stats month: %+v %v", st, err)
	}
	if st, err = repo.Stats(ctx, owner.ID, time.Time{}); err != nil || st.Views != 5 || st.Jizz != 5 {
		t.Fatalf("stats all time: %+v %v", st, err)
	}
	tag := Tag{Name: "prof-tag-" + suffix, Color: "#FFFFFF"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM tags WHERE id = ?", tag.ID) })
	if err := db.Exec("INSERT INTO scene_tags (scene_id, tag_id) VALUES (?, ?), (?, ?)", a.ID, tag.ID, b.ID, tag.ID).Error; err != nil {
		t.Fatal(err)
	}
	tops, err := repo.TopTags(ctx, owner.ID, time.Time{}, 5)
	if err != nil || len(tops) != 1 || tops[0].ID != tag.ID || tops[0].Views != 4 {
		t.Fatalf("top tags: %+v %v", tops, err)
	}
	if _, err := repo.TopActors(ctx, owner.ID, time.Time{}, 5); err != nil {
		t.Fatalf("top actors: %v", err)
	}
	if _, err := repo.TopStudios(ctx, owner.ID, time.Time{}, 5); err != nil {
		t.Fatalf("top studios: %v", err)
	}

	// Playlists: public ones only, cover = first live scene
	pub := Playlist{UserID: owner.ID, Name: "pub", Visibility: "public"}
	priv := Playlist{UserID: owner.ID, Name: "priv", Visibility: "private"}
	for _, pl := range []*Playlist{&pub, &priv} {
		if err := db.Create(pl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO playlist_scenes (playlist_id, scene_id, position) VALUES (?, ?, 0), (?, ?, 1)", pub.ID, trashed.ID, pub.ID, c.ID).Error; err != nil {
		t.Fatal(err)
	}
	pls, plTotal, err := repo.PublicPlaylists(ctx, owner.ID, 10)
	if err != nil || plTotal != 1 || len(pls) != 1 || pls[0].Name != "pub" || pls[0].SceneCount != 1 || pls[0].CoverSceneID == nil || *pls[0].CoverSceneID != c.ID {
		t.Fatalf("public playlists: %+v %d %v", pls, plTotal, err)
	}

	// Deleting the user cascades
	if err := db.Exec("DELETE FROM playlist_scenes WHERE playlist_id = ?", pub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM playlists WHERE user_id = ?", owner.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM users WHERE id = ?", owner.ID).Error; err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Raw("SELECT (SELECT COUNT(*) FROM user_profiles WHERE user_id = ?) + (SELECT COUNT(*) FROM user_profile_favorite_scenes WHERE user_id = ?) + (SELECT COUNT(*) FROM user_profile_favorite_markers WHERE user_id = ?)", owner.ID, owner.ID, owner.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("profile rows left after the user deletion: %d", n)
	}
}
