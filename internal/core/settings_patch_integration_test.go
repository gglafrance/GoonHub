package core

import (
	"os"
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/infrastructure/persistence/migrator"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Partial settings updates against PostgreSQL (GOONHUB_TEST_IMPORT_POSTGRES_DSN, a disposable
// database): absent settings keep their stored value, and settings turned to the zero value of
// a column with a default are really stored on a new row.
func TestUpdateSettings_PostgreSQL(t *testing.T) {
	dsn := os.Getenv("GOONHUB_TEST_IMPORT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_IMPORT_POSTGRES_DSN (disposable database) to run settings integration tests")
	}
	if err := migrator.Run(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test connection: %v", err)
		}
	})

	user := &data.User{Username: "settings-patch-" + uuid.NewString()[:8], Password: "x", Role: "user"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Where("user_id = ?", user.ID).Delete(&data.UserSettings{}).Error; err != nil {
			t.Errorf("cleanup settings: %v", err)
		}
		if err := db.Delete(user).Error; err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})

	repo := data.NewUserSettingsRepository(db)
	svc := NewSettingsService(repo, data.NewUserRepository(db), zap.NewNop())
	stored := func() *data.UserSettings {
		t.Helper()
		s, err := repo.GetByUserID(user.ID)
		if err != nil {
			t.Fatalf("read settings: %v", err)
		}
		return s
	}

	// First update creates the row: zero values of columns with a default must be kept
	off, on := false, true
	if _, err := svc.UpdateSettings(user.ID, WithMarkerThumbnailCycling(false), WithDefaultVolume(0), WithReplayHeatmap(&off, nil)); err != nil {
		t.Fatalf("first patch: %v", err)
	}
	s := stored()
	if s.MarkerThumbnailCycling || s.DefaultVolume != 0 || s.ShowReplayHeatmap {
		t.Fatalf("zero values not stored on the new row: cycling=%v volume=%d heatmap=%v", s.MarkerThumbnailCycling, s.DefaultVolume, s.ShowReplayHeatmap)
	}
	if s.VideosPerPage != 20 || s.DefaultSortOrder != "created_at_desc" || !s.RecordReplayHeatmap || s.DateFormat != "auto" || len(s.HomepageConfig.Tabs) == 0 {
		t.Fatalf("new row does not start from the defaults: %+v", s)
	}

	// A second patch only changes its own fields
	if _, err := svc.UpdateSettings(user.ID, WithScrubber(&on), WithGridDensity("large")); err != nil {
		t.Fatalf("second patch: %v", err)
	}
	s = stored()
	if !s.ShowScrubber || s.GridDensity != "large" {
		t.Fatalf("patched fields not stored: scrubber=%v density=%q", s.ShowScrubber, s.GridDensity)
	}
	if s.MarkerThumbnailCycling || s.DefaultVolume != 0 || s.ShowReplayHeatmap {
		t.Fatalf("absent fields changed: cycling=%v volume=%d heatmap=%v", s.MarkerThumbnailCycling, s.DefaultVolume, s.ShowReplayHeatmap)
	}

	// An invalid patch changes nothing
	if _, err := svc.UpdateSettings(user.ID, WithScrubber(&off), WithGridDensity("tiny")); err == nil {
		t.Fatal("expected an error for an invalid grid density")
	}
	if s = stored(); !s.ShowScrubber {
		t.Fatal("an invalid patch must not change the settings")
	}

	// PUT still replaces the positional settings and keeps the optional ones it leaves out
	autoplay, volume, loop, alc, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, scc := validAllSettingsArgs()
	if _, err := svc.UpdateAllSettings(user.ID, autoplay, volume, loop, alc, false, vpp, sort, tagSort, mtc, hc, pr, sp, paa, pcs, spss, false, "grouped", scc); err != nil {
		t.Fatalf("put: %v", err)
	}
	s = stored()
	if s.DefaultVolume != volume || !s.MarkerThumbnailCycling || !s.ShowScrubber || s.GridDensity != "large" {
		t.Fatalf("put result: volume=%d cycling=%v scrubber=%v density=%q", s.DefaultVolume, s.MarkerThumbnailCycling, s.ShowScrubber, s.GridDensity)
	}
}
