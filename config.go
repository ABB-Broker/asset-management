package main

import (
	"os"
	"strings"
)

// Config holds all environment-driven application settings.
// Override any value by setting the corresponding environment variable.
type Config struct {
	AdminUsername string
	AdminPassword string
	TOTPSecret    string

	// DBDriver selects the GORM database driver.
	// Supported: "sqlite" (default).
	// To use MySQL or PostgreSQL add the gorm driver to go.mod and extend initDB.
	DBDriver string

	// DBDSN is the Data Source Name passed to the GORM driver.
	// SQLite: file path, e.g. "asset_management.db" or ":memory:"
	// MySQL:  "user:pass@tcp(host:port)/dbname?parseTime=True&loc=Local"
	// Postgres: "host=h user=u password=p dbname=d port=5432 sslmode=disable"
	DBDSN string

	// Port is the TCP port the HTTP server listens on.
	Port string

	// Prefork enables Fiber's multi-process prefork mode (one worker per CPU core).
	// Disable for local development or test environments via PREFORK=false.
	Prefork bool
}

func loadConfig() Config {
	return Config{
		AdminUsername: envOrDefault("ADMIN_USERNAME", "admin"),
		AdminPassword: envOrDefault("ADMIN_PASSWORD", "admin123"),
		TOTPSecret:    envOrDefault("TOTP_SECRET", "JBSWY3DPEHPK3PXP"),
		DBDriver:      envOrDefault("DB_DRIVER", "sqlite"),
		DBDSN:         envOrDefault("DB_DSN", "asset_management.db"),
		Port:          envOrDefault("PORT", "8080"),
		Prefork:       envOrDefault("PREFORK", "true") == "true",
	}
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
