package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration test of the presence SQL against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a
// disposable database, see importTestDB).
func TestPresenceRepositoryIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	mk := func(name string) User {
		u := User{Username: "pres-" + name + "-" + suffix, Password: "x", Role: "user"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u
	}
	a, b, c, d := mk("a"), mk("b"), mk("c"), mk("d")
	t.Cleanup(func() {
		if err := db.Where("username LIKE ?", "%-"+suffix).Delete(&User{}).Error; err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	social := NewSocialRepository(db)
	repo := NewPresenceRepository(db)

	// a follows b, a and c are friends, d follows a (not a contact of a)
	if _, err := social.Follow(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := social.Follow(ctx, d.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := social.SendFriendRequest(ctx, a.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := social.AcceptFriendRequest(ctx, c.ID, a.ID); err != nil || !ok {
		t.Fatalf("accept: %v %v", ok, err)
	}
	// c is also followed: listed once
	if _, err := social.Follow(ctx, a.ID, c.ID); err != nil {
		t.Fatal(err)
	}

	contacts, err := repo.Contacts(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 2 || contacts[0].ID != b.ID || contacts[1].ID != c.ID || contacts[0].LastSeenAt != nil || contacts[0].Invisible {
		t.Fatalf("contacts of a: %+v", contacts)
	}

	t1 := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	t0 := t1.Add(-time.Hour)
	if err := repo.SaveLastSeen(ctx, map[uint]time.Time{b.ID: t1, c.ID: t0, 999999999: t1}); err != nil {
		t.Fatalf("save last seen (with an unknown user): %v", err)
	}
	// an older time never moves last seen back
	if err := repo.SaveLastSeen(ctx, map[uint]time.Time{b.ID: t0}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetUser(ctx, b.ID)
	if err != nil || got.LastSeenAt == nil || !got.LastSeenAt.Equal(t1) || got.Username != b.Username {
		t.Fatalf("b: %+v %v", got, err)
	}

	if err := repo.SetInvisible(ctx, c.ID, true, nil); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetUser(ctx, c.ID)
	if err != nil || !got.Invisible || got.LastSeenAt == nil || !got.LastSeenAt.Equal(t0) {
		t.Fatalf("c invisible keeps its last seen: %+v %v", got, err)
	}
	t2 := t1.Add(time.Minute)
	if err := repo.SetInvisible(ctx, d.ID, true, &t2); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetUser(ctx, d.ID)
	if err != nil || !got.Invisible || got.LastSeenAt == nil || !got.LastSeenAt.Equal(t2) {
		t.Fatalf("d: %+v %v", got, err)
	}
	if err := repo.SetInvisible(ctx, d.ID, false, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.GetUser(ctx, d.ID); got.Invisible || got.LastSeenAt == nil {
		t.Fatalf("d visible again: %+v", got)
	}

	if _, err := repo.GetUser(ctx, 999999999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unknown user: %v", err)
	}

	// deleting a user deletes their presence
	if err := db.Delete(&User{}, d.ID).Error; err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&UserPresence{}).Where("user_id = ?", d.ID).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("presence not cascaded: %d %v", n, err)
	}
}
