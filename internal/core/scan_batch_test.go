package core

import (
	"errors"
	"testing"
	"unicode/utf8"

	"goonhub/internal/data"
)

func TestValidScanPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/media/stash/data/Clip – part 1.mp4", true}, // en dash, valid UTF-8
		{"/media/stash/data/vidéo.mp4", true},
		{"/media/stash/data/Clip \x96 part 1.mp4", false}, // Windows-1252 en dash byte
		{"/media/stash/data/\x95 bullet.mp4", false},
	}
	for _, c := range cases {
		if got := validScanPath(c.path); got != c.want {
			t.Errorf("validScanPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// fakeSceneCreator fails any insert containing a path that is not valid UTF-8, like
// PostgreSQL does ("invalid byte sequence for encoding UTF8"), and assigns IDs otherwise.
type fakeSceneCreator struct {
	calls  int
	nextID uint
	stored []string
}

func (f *fakeSceneCreator) CreateInBatches(scenes []*data.Scene, _ int) error {
	f.calls++
	for _, sc := range scenes {
		if !utf8.ValidString(sc.StoredPath) {
			return errors.New(`ERROR: invalid byte sequence for encoding "UTF8": 0x96 (SQLSTATE 22021)`)
		}
	}
	for _, sc := range scenes {
		f.nextID++
		sc.ID = f.nextID
		f.stored = append(f.stored, sc.StoredPath)
	}
	return nil
}

func TestCreateScenesResilient_OneBadRowDoesNotDropTheBatch(t *testing.T) {
	repo := &fakeSceneCreator{}
	scenes := []*data.Scene{
		{StoredPath: "/m/a.mp4"},
		{StoredPath: "/m/b \x96.mp4"},
		{StoredPath: "/m/c.mp4"},
	}
	created, failed, err := createScenesResilient(repo, scenes, 50)
	if err == nil {
		t.Fatal("expected the batch error to be reported")
	}
	if len(created) != 2 || created[0].StoredPath != "/m/a.mp4" || created[1].StoredPath != "/m/c.mp4" {
		t.Fatalf("created = %v, want a and c", created)
	}
	if len(failed) != 1 || failed[0].StoredPath != "/m/b \x96.mp4" {
		t.Fatalf("failed = %v, want b", failed)
	}
	if len(repo.stored) != 2 {
		t.Fatalf("stored %v, want 2 scenes", repo.stored)
	}
	for _, sc := range created {
		if sc.ID == 0 {
			t.Fatalf("created scene %s has no ID", sc.StoredPath)
		}
	}
}

func TestCreateScenesResilient_HappyPathIsOneBatch(t *testing.T) {
	repo := &fakeSceneCreator{}
	scenes := []*data.Scene{{StoredPath: "/m/a.mp4"}, {StoredPath: "/m/b.mp4"}}
	created, failed, err := createScenesResilient(repo, scenes, 50)
	if err != nil || len(failed) != 0 || len(created) != 2 {
		t.Fatalf("created %d, failed %d, err %v", len(created), len(failed), err)
	}
	if repo.calls != 1 {
		t.Fatalf("%d insert calls, want a single batch", repo.calls)
	}
}
