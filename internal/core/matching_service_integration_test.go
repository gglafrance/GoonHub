package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goonhub/internal/core/matching"
	"goonhub/internal/data"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func matchingPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GOONHUB_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOONHUB_TEST_POSTGRES_DSN to run PostgreSQL grouping regressions")
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
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil {
			t.Errorf("roll back test schema: %v", err)
		}
	})
	// A private schema inside a rolled-back transaction protects existing tables.
	schema := "duplication_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	for _, sql := range []string{
		"CREATE SCHEMA " + schema,
		"SET LOCAL search_path TO " + schema,
		`CREATE TABLE scenes (id BIGINT PRIMARY KEY, duration INTEGER NOT NULL DEFAULT 0,
		 trashed_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ)`,
		`INSERT INTO scenes (id, duration) VALUES (10, 10), (20, 20), (30, 30), (40, 40)`,
	} {
		if err := tx.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"000057_create_duplicate_groups.up.sql", "000062_unique_scene_in_duplicate_group.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "infrastructure", "persistence", "migrator", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
	return tx
}

func TestPostgresProcessMatchesReopensClosedGroups(t *testing.T) {
	for _, status := range []string{"resolved", "dismissed"} {
		t.Run(status, func(t *testing.T) {
			db := matchingPostgresTestDB(t)
			if err := db.Exec(`INSERT INTO duplicate_groups (id, status, scene_count, resolved_at)
			 VALUES (7, ?, 2, NOW());`, status).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`INSERT INTO duplicate_group_members (group_id, scene_id) VALUES (7, 20), (7, 40)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`UPDATE scenes SET trashed_at = NOW() WHERE id = 20`).Error; err != nil {
				t.Fatal(err)
			}
			groups := data.NewDuplicateGroupRepository(db)
			svc := NewMatchingService(nil, data.NewSceneRepository(db), groups, nil, zap.NewNop())
			if err := svc.ProcessMatches(10, []matching.MatchResult{
				{SceneID: 40, ConfidenceScore: 0.9, MatchType: "audio"},
				{SceneID: 30, ConfidenceScore: 0.8, MatchType: "visual"},
			}); err != nil {
				t.Fatal(err)
			}
			group, err := groups.GetByIDWithMembers(7)
			if err != nil {
				t.Fatal(err)
			}
			if group.Status != "unresolved" || group.ResolvedAt != nil || len(group.Members) != 4 || group.SceneCount != 4 {
				t.Fatalf("unexpected reopened group: %+v", group)
			}
			if group.BestSceneID == nil || *group.BestSceneID != 40 {
				t.Fatalf("unexpected best scene: %v", group.BestSceneID)
			}
		})
	}
}

func TestPostgresProcessMatchesRollsBackMergedGroups(t *testing.T) {
	db := matchingPostgresTestDB(t)
	for _, sql := range []string{
		`INSERT INTO duplicate_groups (id, status, scene_count) VALUES (3, 'unresolved', 1), (7, 'resolved', 1)`,
		`INSERT INTO duplicate_group_members (group_id, scene_id) VALUES (3, 30), (7, 20)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	groups := data.NewDuplicateGroupRepository(db)
	svc := NewMatchingService(nil, data.NewSceneRepository(db), groups, nil, zap.NewNop())
	// The missing scene makes the FK reject an insertion after groups were merged.
	if err := svc.ProcessMatches(10, []matching.MatchResult{
		{SceneID: 20}, {SceneID: 30}, {SceneID: 999},
	}); err == nil {
		t.Fatal("expected foreign-key violation")
	}
	for _, id := range []uint{3, 7} {
		group, err := groups.GetByIDWithMembers(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(group.Members) != 1 || group.SceneCount != 1 {
			t.Fatalf("group %d was partially merged: %+v", id, group)
		}
	}
	group, err := groups.GetGroupBySceneID(10)
	if err != nil || group != nil {
		t.Fatalf("query membership was not rolled back: group=%v err=%v", group, err)
	}
}
