package data

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Integration test of the message reactions SQL (migration 104) against PostgreSQL
// (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a disposable database, see importTestDB).
func TestMessageReactionsIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	mk := func(name string) User {
		u := User{Username: "rx-" + name + "-" + suffix, Password: "x", Role: "user"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u
	}
	alice, bob := mk("alice"), mk("bob")
	t.Cleanup(func() {
		db.Exec(`DELETE FROM conversations WHERE direct_low_id IN (?) OR direct_high_id IN (?) OR direct_low_id IS NULL`, []uint{alice.ID, bob.ID}, []uint{alice.ID, bob.ID})
		if err := db.Where("username LIKE ?", "%-"+suffix).Delete(&User{}).Error; err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if err := db.Exec(`INSERT INTO user_profiles (user_id, display_name) VALUES (?, 'Bobby') ON CONFLICT (user_id) DO UPDATE SET display_name = 'Bobby'`, bob.ID).Error; err != nil {
		t.Fatalf("profile: %v", err)
	}
	repo := NewMessageRepository(db)
	conv, _, err := repo.FindOrCreateDirect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("conversation: %v", err)
	}
	send := func(body string) *Message {
		id := alice.ID
		m := &Message{ConversationID: conv.ID, SenderID: &id, Body: body, CreatedAt: time.Now().UTC()}
		if err := repo.InsertMessage(ctx, m); err != nil {
			t.Fatalf("insert: %v", err)
		}
		return m
	}
	m1, m2 := send("one"), send("two")

	add := func(msg, user uint, key string, kinds, perUser int, want ReactionAddResult) {
		t.Helper()
		got, err := repo.AddReaction(ctx, msg, user, key, kinds, perUser)
		if err != nil || got != want {
			t.Fatalf("AddReaction(%d, %d, %q) = %v %v, want %v", msg, user, key, got, err, want)
		}
	}
	add(m1.ID, alice.ID, "🔥", 3, 2, ReactionAdded)
	add(m1.ID, alice.ID, "🔥", 3, 2, ReactionExists)
	add(m1.ID, bob.ID, "🔥", 3, 2, ReactionAdded)
	add(m1.ID, bob.ID, ":pog:", 3, 2, ReactionAdded)
	add(m1.ID, bob.ID, "😂", 3, 2, ReactionTooManyForUser)
	add(m1.ID, alice.ID, "😂", 3, 2, ReactionAdded)
	add(m1.ID, alice.ID, "👍", 3, 0, ReactionTooManyKinds) // 3 kinds already
	add(m1.ID, alice.ID, ":pog:", 3, 0, ReactionAdded)    // an existing kind is fine
	add(m2.ID, bob.ID, "🎉", 0, 0, ReactionAdded)

	rows, err := repo.ListReactions(ctx, []uint{m1.ID, m2.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].MessageID != m1.ID || rows[0].Emoji != "🔥" || rows[0].UserID != alice.ID || rows[0].Username != alice.Username {
		t.Fatalf("first row = %+v", rows[0])
	}
	for _, r := range rows {
		if r.UserID == bob.ID && r.DisplayName != "Bobby" {
			t.Fatalf("bob's display name = %+v", r)
		}
	}
	if rows[len(rows)-1].MessageID != m2.ID {
		t.Fatalf("rows not ordered by message: %+v", rows)
	}

	// Concurrent reactions respect the caps (the message row is locked)
	m3 := send("three")
	keys := []string{"😀", "😃", "😄", "😁", "😆", "😅", "🤣", "😊"}
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		go func(user uint, key string) {
			defer wg.Done()
			if _, err := repo.AddReaction(ctx, m3.ID, user, key, 3, 0); err != nil {
				t.Errorf("concurrent add: %v", err)
			}
		}([]uint{alice.ID, bob.ID}[i%2], key)
	}
	wg.Wait()
	var kinds int64
	if err := db.Raw(`SELECT COUNT(DISTINCT emoji) FROM message_reactions WHERE message_id = ?`, m3.ID).Scan(&kinds).Error; err != nil || kinds != 3 {
		t.Fatalf("kinds after concurrent adds = %d %v", kinds, err)
	}

	// Removing
	if ok, err := repo.RemoveReaction(ctx, m1.ID, alice.ID, "😂"); err != nil || !ok {
		t.Fatalf("remove: %v %v", ok, err)
	}
	if ok, err := repo.RemoveReaction(ctx, m1.ID, alice.ID, "😂"); err != nil || ok {
		t.Fatalf("remove again: %v %v", ok, err)
	}

	// A deleted message loses its reactions and takes no new one
	if ok, err := repo.SoftDeleteMessage(ctx, m1.ID, time.Now()); err != nil || !ok {
		t.Fatalf("soft delete: %v %v", ok, err)
	}
	if rows, err := repo.ListReactions(ctx, []uint{m1.ID}); err != nil || len(rows) != 0 {
		t.Fatalf("reactions of a deleted message: %+v %v", rows, err)
	}
	add(m1.ID, alice.ID, "🔥", 0, 0, ReactionMessageGone)
	add(999_999_999, alice.ID, "🔥", 0, 0, ReactionMessageGone)

	// A deleted user's reactions go with the account
	if err := db.Delete(&User{}, bob.ID).Error; err != nil {
		t.Fatalf("delete bob: %v", err)
	}
	if rows, err := repo.ListReactions(ctx, []uint{m2.ID}); err != nil || len(rows) != 0 {
		t.Fatalf("reactions of a deleted user: %+v %v", rows, err)
	}
	// A hard deleted message (retention) takes its reactions with it
	add(m3.ID, alice.ID, "😺", 0, 0, ReactionAdded)
	if err := db.Exec(`DELETE FROM messages WHERE id = ?`, m3.ID).Error; err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	var left int64
	if err := db.Raw(`SELECT COUNT(*) FROM message_reactions WHERE message_id = ?`, m3.ID).Scan(&left).Error; err != nil || left != 0 {
		t.Fatalf("reactions after a hard delete = %d %v", left, err)
	}
}
