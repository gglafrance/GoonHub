package data

import (
	"fmt"
	"testing"
	"time"
)

// Integration test of storage_paths.read_only (migration 000075) through GORM, on the disposable
// import test database (GOONHUB_TEST_IMPORT_POSTGRES_DSN).
func TestStoragePathRepositoryReadOnly(t *testing.T) {
	db := importTestDB(t)
	repo := NewStoragePathRepository(db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	writable := &StoragePath{Name: "Writable", Path: "/read-only-test/" + suffix + "/rw"}
	readOnly := &StoragePath{Name: "Stash", Path: "/read-only-test/" + suffix + "/ro", ReadOnly: true}
	for _, p := range []*StoragePath{writable, readOnly} {
		if err := repo.Create(p); err != nil {
			t.Fatalf("create: %v", err)
		}
		id := p.ID
		t.Cleanup(func() { _ = repo.Delete(id) })
	}

	got, err := repo.GetByID(readOnly.ID)
	if err != nil || got == nil || !got.ReadOnly {
		t.Fatalf("expected the read-only flag to be stored, got %+v, %v", got, err)
	}
	got, err = repo.GetByID(writable.ID)
	if err != nil || got == nil || got.ReadOnly {
		t.Fatalf("expected a writable path by default, got %+v, %v", got, err)
	}

	// Update writes the flag both ways
	got.ReadOnly = true
	if err := repo.Update(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	if again, _ := repo.GetByID(writable.ID); again == nil || !again.ReadOnly {
		t.Fatalf("expected read-only after update, got %+v", again)
	}
	got.ReadOnly = false
	if err := repo.Update(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	if again, _ := repo.GetByID(writable.ID); again == nil || again.ReadOnly {
		t.Fatalf("expected writable after update, got %+v", again)
	}
}
