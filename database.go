package main

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// initDB opens the database connection using the driver specified in cfg and
// runs AutoMigrate for all registered models.
//
// Supported DB_DRIVER values:
//   - "sqlite" (default) — CGO-free SQLite via modernc.org/sqlite
//   - "mysql" — MySQL / MariaDB via gorm.io/driver/mysql
//
// MySQL DB_DSN example:
//
//	user:password@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
func initDB(cfg Config) *gorm.DB {
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "mysql":
		if cfg.DBDSN == "" {
			log.Fatal("DB_DSN must be set when DB_DRIVER=mysql")
		}
		dialector = mysql.Open(cfg.DBDSN)
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

	// Enable foreign-key enforcement for SQLite so that the ON DELETE CASCADE
	// constraint on Asset.CategoryID is honoured at the database level.
	// MySQL enforces FK constraints by default.
	if cfg.DBDriver == "sqlite" || cfg.DBDriver == "" {
		if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			log.Fatalf("enable sqlite foreign keys: %v", err)
		}
	}

	if err := db.AutoMigrate(&Category{}, &Asset{}, &Session{}); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	return db
}
