# asset-management

Assets Management by ABB Insurance Brokers.

## Features
- **Fiber v3** web framework with **Prefork** enabled (one worker per CPU core)
- **GORM** database layer — SQLite by default; swap in MySQL/PostgreSQL via env vars
- Login with **bcrypt** password hashing
- **Two-Factor Authentication** (TOTP — compatible with Google Authenticator / Authy)
- **Category Master** CRUD
- **Asset Master** CRUD (linked to category master with cascade delete)
- **Metis-style** admin sidebar layout

## Quick Start

```bash
go mod tidy
go run .
```

Open `http://localhost:8080`

| Credential | Default value |
|---|---|
| Username | `admin` |
| Password | `admin123` |
| TOTP secret | `JBSWY3DPEHPK3PXP` |

Scan the TOTP secret with Google Authenticator or Authy, or use any RFC 6238 TOTP tool.

## Configuration (environment variables)

| Variable | Default | Description |
|---|---|---|
| `ADMIN_USERNAME` | `admin` | Admin login username |
| `ADMIN_PASSWORD` | `admin123` | Admin login password (hashed with bcrypt at startup) |
| `TOTP_SECRET` | `JBSWY3DPEHPK3PXP` | Base-32 TOTP secret key |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite`, `mysql`; extend `database.go` for postgres) |
| `DB_DSN` | `asset_management.db` | Data source name — SQLite file path **or** MySQL connection string (see below) |
| `PORT` | `8080` | HTTP listen port |
| `PREFORK` | `true` | Enable Fiber Prefork (`true`/`false`) |

## Adding MySQL / MariaDB

Set these environment variables before starting the server:

```bash
export DB_DRIVER=mysql
export DB_DSN="user:password@tcp(host:3306)/asset_management?charset=utf8mb4&parseTime=True&loc=Local"
```

The MySQL driver (`gorm.io/driver/mysql`) is already included in `go.mod`.  
Create the database first: `CREATE DATABASE asset_management CHARACTER SET utf8mb4;`

## Adding PostgreSQL

1. Add the GORM driver to `go.mod`:
   ```
   gorm.io/driver/postgres v1.5.x
   ```
2. Import the driver in `database.go` and add a `case` to the `switch`:
   ```go
   case "postgres":
       dialector = postgres.Open(cfg.DBDSN)
   ```
3. Set `DB_DRIVER=postgres` and `DB_DSN` at runtime.

