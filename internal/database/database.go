package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"time"

	"github.com/ABB-Broker/asset-management/internal/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Init(cfg config.Config) *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Asia%%2FJakarta&multiStatements=true",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		cfg.DBCharset,
	)

	// Run migrations using raw sql.DB first
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open db for migration: %v", err)
	}

	runMigrations(sqlDB, cfg.DBName)

	// Hand the same connection to GORM
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("gorm open: %v", err)
	}

	sqlDB.SetMaxOpenConns(cfg.DBOpenConnection)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConnection)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db
}

func runMigrations(sqlDB *sql.DB, dbName string) {
	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		log.Fatalf("migration source: %v", err)
	}

	dbDriver, err := mysql.WithInstance(sqlDB, &mysql.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		log.Fatalf("migration db driver: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, dbName, dbDriver)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("database migrations applied successfully")
}

func RunDown(cfg config.Config) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Asia%%2FJakarta&multiStatements=true",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		cfg.DBCharset)

	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		log.Fatalf("migration source: %v", err)
	}

	dbDriver, err := mysql.WithInstance(sqlDB, &mysql.Config{
		DatabaseName: cfg.DBName,
	})
	if err != nil {
		log.Fatalf("migration db driver: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, cfg.DBName, dbDriver)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate down failed: %v", err)
	}

	log.Println("all tables dropped")
}
