package postgres

import (
	"fmt"

	"goonhub/internal/config"
	"goonhub/internal/infrastructure/logging"
	"goonhub/internal/infrastructure/persistence/migrator"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewDB(cfg *config.Config, logger *logging.Logger) (*gorm.DB, error) {
	dsn := cfg.Database.DSN()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)

	// Run migrations
	migrationDSN := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName, cfg.Database.SSLMode,
	)

	if err := migrator.Run(migrationDSN); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("PostgreSQL connected and migrations applied")

	return db, nil
}
