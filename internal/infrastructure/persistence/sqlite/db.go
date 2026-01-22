package sqlite

import (
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/infrastructure/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewDB(cfg *config.Config, logger *logging.Logger) (*gorm.DB, error) {
	// GORM Logger using our Zap logger
	// For now, using default GORM logger, but we should wrap Zap later
	// TODO: Wrap Zap logger for GORM

	db, err := gorm.Open(sqlite.Open(cfg.Database.Source), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	})
	if err != nil {
		return nil, err
	}

	// Auto Migrate (Move to migrator later)
	if err := db.AutoMigrate(&data.Video{}, &data.User{}, &data.RevokedToken{}); err != nil {
		logger.Error("Failed to migrate database: " + err.Error())
		return nil, err
	}

	return db, nil
}
