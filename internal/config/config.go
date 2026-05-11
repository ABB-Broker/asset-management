// Package config provides Viper-based configuration loading for the
// asset_management application.
//
// Configuration is resolved in priority order (highest first):
//  1. Environment variables (e.g. ADMIN_PASSWORD=secret)
//  2. config.yaml / config.json / config.toml in the working directory
//  3. Built-in defaults
package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application settings resolved at startup.
type Config struct {
	// AdminUsername is the login username for the single built-in admin account.
	AdminUsername string
	// AdminPassword is the plaintext password (bcrypt-hashed at startup).
	AdminPassword string
	// TOTPSecret is the Base-32 TOTP secret key (RFC 6238).
	TOTPSecret string

	// DBDriver selects the GORM database driver: "sqlite" (default) or "mysql".
	DBDriver string
	// DBDSN is the data source name passed to the selected GORM driver.
	//   SQLite: file path, e.g. "asset_management.db" or ":memory:"
	//   MySQL:  "user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	DBDSN string

	// Port is the TCP port the HTTP server listens on (default "8080").
	Port string
	// Prefork enables Fiber multi-process prefork mode (one worker per CPU core).
	Prefork bool
}

// Load reads configuration from environment variables, an optional config file
// (config.yaml / .env in the working directory), and falls back to defaults.
func Load() Config {
	v := viper.New()

	// ── Defaults ────────────────────────────────────────────────────────────
	v.SetDefault("admin_username", "admin")
	v.SetDefault("admin_password", "admin123")
	v.SetDefault("totp_secret", "353353")
	v.SetDefault("db_driver", "sqlite")
	v.SetDefault("db_dsn", "asset_management.db")
	v.SetDefault("port", "8080")
	v.SetDefault("prefork", true)

	// ── Config file (optional) ───────────────────────────────────────────────
	// Place a config.yaml (or .env, config.json, config.toml) in the working
	// directory to override defaults without setting environment variables.
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		// Config file is optional — ignore "not found" errors.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("config: warning: %v", err)
		}
	}

	// ── Environment variables ────────────────────────────────────────────────
	// Viper maps env vars to config keys via the key replacer (dots/dashes → underscores).
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	return Config{
		AdminUsername: v.GetString("admin_username"),
		AdminPassword: v.GetString("admin_password"),
		TOTPSecret:    v.GetString("totp_secret"),
		DBDriver:      v.GetString("db_driver"),
		DBDSN:         v.GetString("db_dsn"),
		Port:          v.GetString("port"),
		Prefork:       v.GetBool("prefork"),
	}
}
