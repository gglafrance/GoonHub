package data

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Integration test of the messages SQL against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a
// disposable database, see importTestDB).
func TestMessageRepositoryIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	mk := func(name string, share bool) User {
		u := User{Username: "msg-" + name + "-" + suffix, Password: "x", Role: "user", ShareMarkers: share}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u
	}
	alice, bob, carol := mk("alice", false), mk("bob", false), mk("carol", true)
	base := "/msg-test/" + suffix + "/"
	t.Cleanup(func() {
		db.Exec(`DELETE FROM conversations WHERE direct_low_id IN (?) OR direct_high_id IN (?) OR direct_low_id IS NULL`, []uint{alice.ID, bob.ID, carol.ID}, []uint{alice.ID, bob.ID, carol.ID})
		db.Where("user_id IN ?", []uint{alice.ID, bob.ID, carol.ID}).Delete(&UserSceneMarker{})
		db.Where("user_id IN ?", []uint{alice.ID, bob.ID, carol.ID}).Delete(&Playlist{})
		db.Unscoped().Where("stored_path LIKE ?", base+"%").Delete(&Scene{})
		if err := db.Where("username LIKE ?", "%-"+suffix).Delete(&User{}).Error; err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	repo := NewMessageRepository(db)

	// One conversation per pair, also when created concurrently and in both orders
	var wg sync.WaitGroup
	convs := make([]*Conversation, 6)
	for i := range convs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, b := alice.ID, bob.ID
			if i%2 == 1 {
				a, b = b, a
			}
			c, _, err := repo.FindOrCreateDirect(ctx, a, b)
			if err != nil {
				t.Errorf("find or create: %v", err)
				return
			}
			convs[i] = c
		}(i)
	}
	wg.Wait()
	for _, c := range convs {
		if c == nil || c.ID != convs[0].ID {
			t.Fatalf("several conversations for one pair: %+v", convs)
		}
	}
	conv := convs[0]
	if ids, err := repo.ParticipantIDs(ctx, conv.ID); err != nil || len(ids) != 2 {
		t.Fatalf("participants: %v %v", ids, err)
	}
	if found, err := repo.FindDirect(ctx, bob.ID, alice.ID); err != nil || found.ID != conv.ID {
		t.Fatalf("find direct: %v %v", found, err)
	}

	// Messages: alice sends 3, bob 1
	send := func(from uint, body string) *Message {
		id := from
		m := &Message{ConversationID: conv.ID, SenderID: &id, Body: body, CreatedAt: time.Now().UTC()}
		if err := repo.InsertMessage(ctx, m); err != nil {
			t.Fatalf("insert: %v", err)
		}
		return m
	}
	m1 := send(alice.ID, "one")
	m2 := send(alice.ID, "two")
	m3 := send(alice.ID, "three")
	m4 := send(bob.ID, "four")

	// bob read his own message (the sender has read it), so nothing unread for him... but
	// m1-m3 came before his last read: unread counts only after the last read message
	sum, err := repo.UnreadSummary(ctx, bob.ID)
	if err != nil || sum.Messages != 0 {
		t.Fatalf("bob unread after answering: %+v %v", sum, err)
	}
	sum, err = repo.UnreadSummary(ctx, alice.ID)
	if err != nil || sum.Messages != 1 || sum.Conversations != 1 {
		t.Fatalf("alice unread: %+v %v", sum, err)
	}

	// Pagination, newest first
	page, err := repo.ListMessages(ctx, conv.ID, 0, 2)
	if err != nil || len(page) != 2 || page[0].ID != m4.ID || page[1].ID != m3.ID {
		t.Fatalf("first page: %+v %v", page, err)
	}
	page, err = repo.ListMessages(ctx, conv.ID, m3.ID, 10)
	if err != nil || len(page) != 2 || page[0].ID != m2.ID || page[1].ID != m1.ID {
		t.Fatalf("older page: %+v %v", page, err)
	}

	// Read state only moves forward, and only to a message of the conversation
	if changed, err := repo.MarkRead(ctx, conv.ID, alice.ID, m4.ID); err != nil || !changed {
		t.Fatalf("mark read: %v %v", changed, err)
	}
	if changed, err := repo.MarkRead(ctx, conv.ID, alice.ID, m2.ID); err != nil || changed {
		t.Fatalf("mark read backwards: %v %v", changed, err)
	}
	if sum, _ := repo.UnreadSummary(ctx, alice.ID); sum.Messages != 0 {
		t.Fatalf("alice unread after reading: %+v", sum)
	}

	// Edit, soft delete (body and attachment cleared), unread ignores deleted messages
	if ok, err := repo.UpdateMessageBody(ctx, m1.ID, "one!", time.Now()); err != nil || !ok {
		t.Fatalf("edit: %v %v", ok, err)
	}
	m5 := send(bob.ID, "five")
	if ok, err := repo.SoftDeleteMessage(ctx, m5.ID, time.Now()); err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, err := repo.SoftDeleteMessage(ctx, m5.ID, time.Now()); err != nil || ok {
		t.Fatalf("delete twice: %v %v", ok, err)
	}
	if ok, err := repo.UpdateMessageBody(ctx, m5.ID, "x", time.Now()); err != nil || ok {
		t.Fatalf("edit a deleted message: %v %v", ok, err)
	}
	if sum, _ := repo.UnreadSummary(ctx, alice.ID); sum.Messages != 0 {
		t.Fatalf("a deleted message counted unread: %+v", sum)
	}

	// Conversation rows: other participant, last message (deleted), read states
	rows, total, err := repo.ListConversations(ctx, alice.ID, 1, 10)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("list: %v %d %v", rows, total, err)
	}
	row := rows[0]
	if row.OtherUserID == nil || *row.OtherUserID != bob.ID || row.OtherUsername != bob.Username || row.LastID == nil || *row.LastID != m5.ID || row.LastDeletedAt == nil {
		t.Fatalf("row = %+v", row)
	}
	if row.OtherLastReadMessageID == nil || *row.OtherLastReadMessageID != m5.ID || row.LastReadMessageID == nil || *row.LastReadMessageID != m4.ID {
		t.Fatalf("read states = %+v", row)
	}
	if _, err := repo.GetConversationRow(ctx, carol.ID, conv.ID); err == nil {
		t.Fatal("a non participant read the conversation")
	}

	// Mute: excluded from the unread summary
	send(bob.ID, "six")
	if err := repo.SetMuted(ctx, conv.ID, alice.ID, true); err != nil {
		t.Fatalf("mute: %v", err)
	}
	if sum, _ := repo.UnreadSummary(ctx, alice.ID); sum.Messages != 0 {
		t.Fatalf("a muted conversation counted: %+v", sum)
	}
	if r, _ := repo.GetConversationRow(ctx, alice.ID, conv.ID); r == nil || !r.Muted || r.UnreadCount != 1 {
		t.Fatalf("muted row = %+v", r)
	}

	// Settings: defaults, then partial updates
	st, err := repo.GetSettings(ctx, alice.ID)
	if err != nil || st.Policy != MessagePolicyFriends || !st.Toasts {
		t.Fatalf("default settings: %+v %v", st, err)
	}
	everyone := MessagePolicyEveryone
	if err := repo.SaveSettings(ctx, alice.ID, &everyone, nil); err != nil {
		t.Fatalf("save policy: %v", err)
	}
	off := false
	if err := repo.SaveSettings(ctx, alice.ID, nil, &off); err != nil {
		t.Fatalf("save toasts: %v", err)
	}
	if st, _ := repo.GetSettings(ctx, alice.ID); st.Policy != MessagePolicyEveryone || st.Toasts {
		t.Fatalf("settings = %+v", st)
	}
	social := NewSocialRepository(db)
	if rels, err := social.Relations(ctx, bob.ID, []uint{alice.ID, carol.ID}); err != nil || len(rels) != 2 {
		t.Fatalf("relations: %v %v", rels, err)
	} else {
		for _, r := range rels {
			want := MessagePolicyFriends
			if r.UserID == alice.ID {
				want = MessagePolicyEveryone
			}
			if r.MessagePolicy != want {
				t.Fatalf("message policy of %d = %q, want %q", r.UserID, r.MessagePolicy, want)
			}
		}
	}
	if err := db.Exec(`UPDATE user_social_settings SET message_policy = 'bogus' WHERE user_id = ?`, alice.ID).Error; err == nil {
		t.Fatal("an unknown message policy was accepted by the schema")
	}

	// Cards: scene (trashed), marker (visibility), playlist (count and thumbnail)
	trashed := time.Now()
	scene := &Scene{Title: "Shared", StoredPath: base + "a.mp4", Duration: 600, ThumbnailPath: "a.jpg", Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	inTrash := &Scene{Title: "Trashed", StoredPath: base + "t.mp4", Duration: 60, TrashedAt: &trashed, Tags: pq.StringArray{}, Actors: pq.StringArray{}}
	for _, s := range []*Scene{scene, inTrash} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("create scene: %v", err)
		}
	}
	scenes, err := repo.SceneCards(ctx, []uint{scene.ID, inTrash.ID, 999999999})
	if err != nil || len(scenes) != 2 {
		t.Fatalf("scene cards: %+v %v", scenes, err)
	}
	for _, s := range scenes {
		if (s.ID == inTrash.ID) != (s.TrashedAt != nil) || (s.ID == scene.ID && (s.Title != "Shared" || s.Duration != 600 || s.ThumbnailPath != "a.jpg")) {
			t.Fatalf("scene card = %+v", s)
		}
	}
	end := 40
	aliceMarker := &UserSceneMarker{UserID: alice.ID, SceneID: scene.ID, Timestamp: 10, EndTimestamp: &end, Label: "Kiss", Color: "#FFFFFF", ThumbnailPath: "m.webp"}
	carolMarker := &UserSceneMarker{UserID: carol.ID, SceneID: scene.ID, Timestamp: 20, Label: "Hug", Color: "#FFFFFF"}
	for _, m := range []*UserSceneMarker{aliceMarker, carolMarker} {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create marker: %v", err)
		}
	}
	if ok, _ := repo.MarkerVisibleTo(ctx, aliceMarker.ID, bob.ID); ok {
		t.Fatal("bob sees a marker alice does not share")
	}
	if ok, _ := repo.MarkerVisibleTo(ctx, carolMarker.ID, bob.ID); !ok {
		t.Fatal("bob does not see a shared marker")
	}
	if ok, _ := repo.MarkerVisibleTo(ctx, aliceMarker.ID, alice.ID); !ok {
		t.Fatal("alice does not see her marker")
	}
	markers, err := repo.MarkerCards(ctx, []uint{aliceMarker.ID})
	if err != nil || len(markers) != 1 || markers[0].SceneTitle != "Shared" || markers[0].EndTimestamp == nil || *markers[0].EndTimestamp != 40 || markers[0].OwnerUsername != alice.Username {
		t.Fatalf("marker cards: %+v %v", markers, err)
	}

	playlists := NewPlaylistRepository(db)
	pl := &Playlist{UserID: alice.ID, Name: "Mine " + suffix, Visibility: "private"}
	if err := playlists.Create(pl); err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	if err := playlists.AddScenes(pl.ID, []uint{inTrash.ID, scene.ID}); err != nil {
		t.Fatalf("add scenes: %v", err)
	}
	if id, err := repo.PlaylistIDByUUID(ctx, pl.UUID); err != nil || id != pl.ID {
		t.Fatalf("playlist by uuid: %d %v", id, err)
	}
	cards, err := repo.PlaylistCards(ctx, []uint{pl.ID})
	if err != nil || len(cards) != 1 {
		t.Fatalf("playlist cards: %+v %v", cards, err)
	}
	if c := cards[0]; c.ItemCount != 1 || c.ThumbnailSceneID == nil || *c.ThumbnailSceneID != scene.ID || c.OwnerUsername != alice.Username || c.Kind != PlaylistKindScenes {
		t.Fatalf("playlist card = %+v", c)
	}

	// A message sharing the scene: deleting the scene keeps the message, the reference goes
	kind := AttachmentScene
	sid := scene.ID
	share := &Message{ConversationID: conv.ID, SenderID: &bob.ID, AttachmentKind: &kind, AttachmentSceneID: &sid}
	if err := repo.InsertMessage(ctx, share); err != nil {
		t.Fatalf("insert share: %v", err)
	}
	if err := db.Exec(`DELETE FROM user_scene_markers WHERE scene_id = ?`, scene.ID).Error; err != nil {
		t.Fatalf("delete markers: %v", err)
	}
	if err := db.Unscoped().Delete(&Scene{}, scene.ID).Error; err != nil {
		t.Fatalf("delete scene: %v", err)
	}
	got, err := repo.GetMessage(ctx, share.ID)
	if err != nil || got.AttachmentKind == nil || *got.AttachmentKind != AttachmentScene || got.AttachmentSceneID != nil {
		t.Fatalf("share after the scene deletion: %+v %v", got, err)
	}

	// Deleting a user keeps the other side's history: sender NULL, conversation kept
	if err := db.Delete(&User{}, bob.ID).Error; err != nil {
		t.Fatalf("delete bob: %v", err)
	}
	got, err = repo.GetMessage(ctx, m4.ID)
	if err != nil || got.SenderID != nil {
		t.Fatalf("bob's message after deletion: %+v %v", got, err)
	}
	row2, err := repo.GetConversationRow(ctx, alice.ID, conv.ID)
	if err != nil || row2.OtherUserID != nil {
		t.Fatalf("conversation after deletion: %+v %v", row2, err)
	}
	// Once alice is gone too, the conversation is an orphan
	if err := db.Delete(&User{}, alice.ID).Error; err != nil {
		t.Fatalf("delete alice: %v", err)
	}
	if n, err := repo.DeleteOrphanConversations(ctx); err != nil || n < 1 {
		t.Fatalf("orphans: %d %v", n, err)
	}
	if _, err := repo.GetConversation(ctx, conv.ID); err == nil {
		t.Fatal("orphan conversation kept")
	}

	// Retention purge
	c2, _, err := repo.FindOrCreateDirect(ctx, carol.ID, carol.ID+1_000_000)
	if err == nil {
		t.Fatalf("conversation with an unknown user created: %+v", c2)
	}
	if _, err := repo.PurgeBefore(ctx, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatalf("purge: %v", err)
	}
}
