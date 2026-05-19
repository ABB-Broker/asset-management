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
	// AppEnv is the environment (value either "development" or "production")
	AppEnv string
	// TOTPSecret is the Base-32 TOTP secret key (RFC 6238).
	TOTPSecret string
	// DevOTPBypass is the bypass otp value.
	DevOTPBypass string

	// DBDriver selects the GORM database driver: "sqlite" (default) or "mysql".
	DBDriver string

	// DBOpenConnection is the maximum number of open connections to the database.
	DBOpenConnection int

	// DBMaxIdleConnection is the maximum number of idle connections in the pool.
	DBMaxIdleConnection int

	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     int
	DBName     string
	DBCharset  string

	// DBDSN is the data source name passed to the selected GORM driver.
	//   SQLite: file path, e.g. "asset_management.db" or ":memory:"
	//   MySQL:  "user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	DBDSN string

	// Port is the TCP port the HTTP server listens on (default "8080").
	Port string
	// BaseURL is the url of the site
	BaseURL string `yaml:"base_url"`

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
	v.SetDefault("app_env", "development")
	v.SetDefault("totp_secret", "JBSWY3DPEHPK3PXP")
	v.SetDefault("dev_otp_bypass", "353353")
	v.SetDefault("db_driver", "sqlite")
	v.SetDefault("db_dsn", "asset_management.db")
	v.SetDefault("db_open_connection", 250)
	v.SetDefault("db_max_idle_connection", 12)
	v.SetDefault("port", "8080")
	v.SetDefault("prefork", true)
	v.SetDefault("base_url", "http://localhost:2005")

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
		AdminUsername:       v.GetString("admin_username"),
		AdminPassword:       v.GetString("admin_password"),
		AppEnv:              v.GetString("app_env"),
		TOTPSecret:          v.GetString("totp_secret"),
		DevOTPBypass:        v.GetString("dev_otp_bypass"),
		DBDriver:            v.GetString("db_driver"),
		DBDSN:               v.GetString("db_dsn"),
		DBOpenConnection:    v.GetInt("db_open_connection"),
		DBMaxIdleConnection: v.GetInt("db_max_idle_connection"),
		DBUser:              v.GetString("db_username"),
		DBPassword:          v.GetString("db_password"),
		DBHost:              v.GetString("db_host"),
		DBPort:              v.GetInt("db_port"),
		DBName:              v.GetString("db_database"),
		DBCharset:           v.GetString("db_charset"),
		Port:                v.GetString("port"),
		Prefork:             v.GetBool("prefork"),
	}
}
