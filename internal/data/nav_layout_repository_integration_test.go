package data

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// Integration test of the header layouts against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a
// disposable database, see importTestDB): no row, insert, replace, delete.
func TestNavLayoutRepositoryIntegration(t *testing.T) {
	db := importTestDB(t)
	ctx := context.Background()
	u := User{Username: "nav-layout-" + uuid.NewString()[:8], Password: "x", Role: "user"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Delete(&u).Error; err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	repo := NewNavLayoutRepository(db)

	got, err := repo.Get(ctx, u.ID)
	if err != nil || got != nil {
		t.Fatalf("no row: layout %+v, err %v", got, err)
	}

	layout := NavLayout{
		Version: 1,
		Bars:    NavLayoutBars{Top: NavBarZones{Left: []string{"logo"}, Center: []string{"folder:mine"}}},
		Folders: []NavFolder{{ID: "mine", Name: "Mine", Icon: "heroicons:star", Items: []string{"search"}}},
	}
	if err := repo.Save(ctx, u.ID, layout); err != nil {
		t.Fatal(err)
	}
	layout.Normalize()
	got, err = repo.Get(ctx, u.ID)
	if err != nil || got == nil || !reflect.DeepEqual(*got, layout) {
		t.Fatalf("after insert: %+v, err %v, want %+v", got, err, layout)
	}

	layout.Hidden = []string{"history"}
	if err := repo.Save(ctx, u.ID, layout); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err = repo.Get(ctx, u.ID)
	if err != nil || got == nil || !reflect.DeepEqual(got.Hidden, []string{"history"}) {
		t.Fatalf("after replace: %+v, err %v", got, err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.Get(ctx, u.ID); err != nil || got != nil {
		t.Fatalf("after delete: %+v, err %v", got, err)
	}
}
