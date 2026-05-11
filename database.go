package main

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// initDB opens the database connection using the driver specified in cfg and
// runs AutoMigrate for all registered models.
//
// Supported DB_DRIVER values:
//   - "sqlite" (default) — CGO-free SQLite via modernc.org/sqlite
//
// To add MySQL or PostgreSQL support:
//  1. Add the driver to go.mod:
//     gorm.io/driver/mysql v1.5.x
//     gorm.io/driver/postgres v1.5.x
//  2. Import the driver package in this file.
//  3. Add a case to the switch below, e.g.:
//     case "mysql":  dialector = mysql.Open(cfg.DBDSN)
//     case "postgres": dialector = postgres.Open(cfg.DBDSN)
func initDB(cfg Config) *gorm.DB {
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "sqlite":
		dsn := cfg.DBDSN
		if dsn == "" {
			dsn = "asset_management.db"
		}
		dialector = sqlite.Open(dsn)
	default:
		log.Printf("warning: unknown DB_DRIVER %q, falling back to sqlite", cfg.DBDriver)
		dialector = sqlite.Open("asset_management.db")
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("database connect failed [driver=%s]: %v", cfg.DBDriver, err)
	}

	if err := db.AutoMigrate(&Category{}, &Asset{}, &Session{}); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	return db
}
