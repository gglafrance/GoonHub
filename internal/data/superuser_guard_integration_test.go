package data

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"

	"goonhub/internal/infrastructure/persistence/migrator"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// isolatedTestDB creates a fresh database next to GOONHUB_TEST_IMPORT_POSTGRES_DSN (the
// superuser count must not see other tests' users), migrates it, and drops it at the end.
func isolatedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GOONHUB_TEST_IMPORT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database) to run repository integration tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	name := "goonhub_guard_" + uuid.NewString()[:8]
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		_ = adminSQL.Close()
		t.Skipf("cannot create an isolated database: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	isolated := u.String()
	if err := migrator.Run(isolated); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := gorm.Open(postgres.Open(isolated), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		if err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)).Error; err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
		_ = adminSQL.Close()
	})
	return db
}

func TestUserRepository_LastSuperuserGuard(t *testing.T) {
	db := isolatedTestDB(t)
	repo := NewUserRepository(db)

	a := &User{Username: "admin-a", Password: "x", Role: "admin"}
	b := &User{Username: "admin-b", Password: "x", Role: "admin"}
	v := &User{Username: "viewer", Password: "x", Role: "user"}
	for _, u := range []*User{a, b, v} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create %s: %v", u.Username, err)
		}
	}

	// Two superusers demoted at the same time: exactly one succeeds
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, u := range []*User{a, b} {
		wg.Add(1)
		go func(i int, id uint) {
			defer wg.Done()
			errs[i] = repo.UpdateRole(id, "user")
		}(i, u.ID)
	}
	wg.Wait()
	var ok, refused int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrLastSuperuser):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || refused != 1 {
		t.Fatalf("expected one demotion and one refusal, got %v", errs)
	}
	var superusers int64
	db.Raw(`SELECT COUNT(*) FROM users u JOIN roles r ON r.name = u.role WHERE r.is_superuser`).Scan(&superusers)
	if superusers != 1 {
		t.Fatalf("%d superusers left, want 1", superusers)
	}

	var last User
	db.Raw(`SELECT u.* FROM users u JOIN roles r ON r.name = u.role WHERE r.is_superuser`).Scan(&last)
	if err := repo.Delete(last.ID); !errors.Is(err, ErrLastSuperuser) {
		t.Fatalf("deleting the last superuser: %v", err)
	}
	if err := repo.UpdateRole(last.ID, "admin"); err != nil {
		t.Fatalf("keeping a superuser role: %v", err)
	}
	// regular users are not concerned
	if err := repo.UpdateRole(v.ID, "moderator"); err != nil {
		t.Fatalf("regular role change: %v", err)
	}
	if err := repo.Delete(v.ID); err != nil {
		t.Fatalf("regular delete: %v", err)
	}
	// with another superuser, the former last one can go
	c := &User{Username: "admin-c", Password: "x", Role: "admin"}
	if err := db.Create(c).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(last.ID); err != nil {
		t.Fatalf("delete with another superuser left: %v", err)
	}
}
