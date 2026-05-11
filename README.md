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
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite`; extend `database.go` for mysql/postgres) |
| `DB_DSN` | `asset_management.db` | Data source name (SQLite file path, or RDBMS connection string) |
| `PORT` | `8080` | HTTP listen port |
| `PREFORK` | `true` | Enable Fiber Prefork (`true`/`false`) |

## Adding MySQL / PostgreSQL

1. Add the GORM driver to `go.mod`:
   ```
   gorm.io/driver/mysql v1.5.x
   gorm.io/driver/postgres v1.5.x
   ```
2. Import the driver in `database.go` and add a `case` to the `switch`:
   ```go
   case "mysql":
       dialector = mysql.Open(cfg.DBDSN)
   case "postgres":
       dialector = postgres.Open(cfg.DBDSN)
   ```
3. Set `DB_DRIVER` and `DB_DSN` environment variables at runtime.

