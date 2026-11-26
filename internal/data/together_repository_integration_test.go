package data

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Integration test of the Goon2gether repositories (migration 098). It reuses the import test
// database (GOONHUB_TEST_IMPORT_POSTGRES_DSN, disposable) and fixtures.
func TestLobbyRepository(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewLobbyRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	other := User{Username: "lobby-other-" + suffix, Password: "x", Role: "admin"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&other) })
	scenes := []*Scene{
		{Title: "Lobby A", Duration: 100, StoredPath: "/import-test/" + suffix + "/lobby-a.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}},
		{Title: "Lobby B", Duration: 50, StoredPath: "/import-test/" + suffix + "/lobby-b.mp4", Tags: pq.StringArray{}, Actors: pq.StringArray{}},
	}
	for _, s := range scenes {
		if err := db.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	host := f.user.ID
	lobby := &Lobby{Name: "Repo test", Visibility: LobbyVisibilityPrivate, HostUserID: &host, CreatedBy: &host,
		InviteCode: "T" + suffix[:7], PlaybackRate: 1, LastActiveAt: now}
	start, end := 5.0, 20.0
	label := "Marker"
	items := []LobbyQueueItem{
		{SceneID: scenes[0].ID, Status: QueueItemQueued, Position: 0, AddedBy: &host},
		{SceneID: scenes[1].ID, Status: QueueItemQueued, Position: 1, StartS: &start, EndS: &end, Label: &label, AddedBy: &host},
	}
	if err := repo.Create(lobby, &LobbyMember{UserID: host, CanControl: true, JoinedAt: now}, items); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(lobby.ID) })
	if lobby.ID == 0 || lobby.UUID == uuid.Nil || items[0].ID == 0 || items[1].LobbyID != lobby.ID {
		t.Fatalf("ids not filled: %+v %+v", lobby, items)
	}
	// Invite codes are unique
	dup := &Lobby{Name: "Dup", Visibility: LobbyVisibilityPublic, InviteCode: lobby.InviteCode, PlaybackRate: 1, LastActiveAt: now}
	if err := repo.Create(dup, nil, nil); !errors.Is(err, ErrInviteCodeTaken) {
		t.Fatalf("duplicate invite code: %v", err)
	}

	got, err := repo.GetByUUID(lobby.UUID)
	if err != nil || got.Name != "Repo test" {
		t.Fatalf("get by uuid: %+v %v", got, err)
	}
	if got, err := repo.GetByInviteCode(lobby.InviteCode); err != nil || got.ID != lobby.ID {
		t.Fatalf("get by code: %v", err)
	}

	// Queue: titles, durations and adder names come along
	q, err := repo.ListQueue(lobby.ID)
	if err != nil || len(q) != 2 || q[0].SceneTitle != "Lobby A" || q[0].SceneDuration != 100 || q[0].AddedByName != f.user.Username {
		t.Fatalf("queue: %+v %v", q, err)
	}
	if *q[1].StartS != 5 || *q[1].EndS != 20 || *q[1].Label != "Marker" {
		t.Fatalf("range item: %+v", q[1])
	}
	sug := []LobbyQueueItem{{LobbyID: lobby.ID, SceneID: scenes[0].ID, Status: QueueItemSuggested, AddedBy: &other.ID}}
	if err := repo.AddQueueItems(sug); err != nil || sug[0].ID == 0 {
		t.Fatalf("add suggestion: %v", err)
	}
	if err := repo.SetQueueOrder(lobby.ID, []uint{items[1].ID, sug[0].ID, items[0].ID}); err != nil {
		t.Fatal(err)
	}
	q, _ = repo.ListQueue(lobby.ID)
	if len(q) != 3 || q[0].ID != items[1].ID || q[1].ID != sug[0].ID || q[1].Status != QueueItemQueued || q[2].ID != items[0].ID {
		t.Fatalf("order: %+v", q)
	}

	// The current item is cleared when its row goes
	if err := repo.SavePlayback(lobby.ID, &items[1].ID, 12.5, 1.5); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByUUID(lobby.UUID)
	if got.CurrentItemID == nil || *got.CurrentItemID != items[1].ID || got.PositionS != 12.5 || got.PlaybackRate != 1.5 {
		t.Fatalf("playback: %+v", got)
	}
	if err := repo.DeleteQueueItem(lobby.ID, items[1].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.GetByUUID(lobby.UUID); got.CurrentItemID != nil {
		t.Fatalf("current item not cleared: %v", *got.CurrentItemID)
	}

	// Settings and host
	got.Name, got.Visibility, got.ControlOpen, got.MaxMembers = "Renamed", LobbyVisibilityPublic, true, 4
	if err := repo.UpdateSettings(got); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetHost(lobby.ID, &other.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountHostedBy(other.ID); n != 1 {
		t.Fatalf("hosted by other: %d", n)
	}
	if err := repo.SetInviteCode(lobby.ID, "U"+suffix[:7]); err != nil {
		t.Fatal(err)
	}

	// Members: add (idempotent), control, list with usernames, ban, remove
	m := &LobbyMember{LobbyID: lobby.ID, UserID: other.ID}
	if err := repo.AddMember(m); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddMember(&LobbyMember{LobbyID: lobby.ID, UserID: other.ID, CanControl: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetMemberControl(lobby.ID, other.ID, true); err != nil {
		t.Fatal(err)
	}
	members, err := repo.ListMembers(lobby.ID)
	if err != nil || len(members) != 2 || members[1].Username != other.Username || !members[1].CanControl {
		t.Fatalf("members: %+v %v", members, err)
	}

	// Listing: public now, with the host name, the item playing and the membership
	if err := repo.SavePlayback(lobby.ID, &items[0].ID, 0, 1); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(other.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var entry *LobbyListEntry
	for i := range list {
		if list[i].ID == lobby.ID {
			entry = &list[i]
		}
	}
	if entry == nil || !entry.IsMember || entry.HostUsername != other.Username || entry.QueueLength != 2 ||
		entry.NowSceneID == nil || *entry.NowSceneID != scenes[0].ID || entry.NowSceneTitle != "Lobby A" {
		t.Fatalf("list entry: %+v", entry)
	}

	if err := repo.BanMember(lobby.ID, other.ID, now); err != nil {
		t.Fatal(err)
	}
	if mm, err := repo.GetMember(lobby.ID, other.ID); err != nil || mm.BannedAt == nil || mm.CanControl {
		t.Fatalf("ban: %+v %v", mm, err)
	}
	list, _ = repo.List(other.ID, false)
	for _, e := range list {
		if e.ID == lobby.ID && (e.IsMember || e.MemberBannedAt == nil) {
			t.Fatalf("a banned user is still a member: %+v", e)
		}
	}
	if err := repo.RemoveMember(lobby.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetMember(lobby.ID, other.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("member not removed: %v", err)
	}

	// Idle cleanup keeps the loaded lobbies and recent ones
	old := now.Add(-48 * time.Hour)
	if err := repo.Touch([]uint{lobby.ID}, old); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.DeleteIdle(now.Add(-24*time.Hour), []uint{lobby.ID}); err != nil || n != 0 {
		t.Fatalf("kept lobby deleted: %d %v", n, err)
	}
	if n, err := repo.DeleteIdle(now.Add(-24*time.Hour), nil); err != nil || n < 1 {
		t.Fatalf("idle lobby not deleted: %d %v", n, err)
	}
	var left int64
	db.Model(&LobbyQueueItem{}).Where("lobby_id = ?", lobby.ID).Count(&left)
	if left != 0 {
		t.Fatalf("queue items left after the lobby deletion: %d", left)
	}
}

func TestEmoteRepository(t *testing.T) {
	db := importTestDB(t)
	f := newImportFixture(t, db)
	repo := NewEmoteRepository(db)
	suffix := f.tags[0].Name[len("import-a-"):]

	e := &Emote{Name: "Pog_" + suffix, Filename: suffix + "0000000000000000000000000a.png", ContentType: "image/png", Width: 32, Height: 32, SizeBytes: 100, CreatedBy: &f.user.ID}
	if err := repo.Create(e); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&Emote{}, e.ID) })
	clash := &Emote{Name: "POG_" + suffix, Filename: suffix + "0000000000000000000000000b.png", ContentType: "image/png", Width: 1, Height: 1, SizeBytes: 1}
	if err := repo.Create(clash); !errors.Is(err, ErrEmoteNameTaken) {
		t.Fatalf("case-insensitive name clash: %v", err)
	}
	bad := &Emote{Name: "bad name", Filename: suffix + "0000000000000000000000000c.png", ContentType: "image/png", Width: 1, Height: 1, SizeBytes: 1}
	if err := repo.Create(bad); err == nil {
		db.Delete(&Emote{}, bad.ID)
		t.Fatal("the name check constraint did not refuse a space")
	}
	if err := repo.Rename(e.ID, "Kappa_"+suffix); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetByID(e.ID); err != nil || got.Name != "Kappa_"+suffix {
		t.Fatalf("rename: %+v %v", got, err)
	}
	if err := repo.Rename(999999, "x_"+suffix); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("rename unknown: %v", err)
	}
	if n, _ := repo.Count(); n < 1 {
		t.Fatalf("count %d", n)
	}
	if err := repo.Delete(e.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(e.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}
