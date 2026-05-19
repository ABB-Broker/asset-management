# asset-management

Assets Management by ABB Insurance Brokers.

## Features
- **Fiber v3.2.0** web framework with **Prefork** enabled (one worker per CPU core)
- **GORM** database layer — SQLite by default; MySQL supported via env var
- **Viper** configuration — env vars, optional `config.yaml`, with sensible defaults
- Login with **bcrypt** password hashing
- **Two-Factor Authentication** (TOTP — compatible with Google Authenticator / Authy)
- **User Master** CRUD (manage application user accounts with role-based access)
- **Category Master** CRUD
- **Asset Master** CRUD (linked to category master with cascade delete)
- **Gentelella Bootstrap Admin Template** dark sidebar UI
- **Zap structured logger** middleware (`gofiber/contrib/v3/zap`)
- **Swagger API documentation** at `/swagger/` (`gofiber/contrib/v3/swaggo`)
- **i18n multi-language** support — English (`en`) and Indonesian (`id`) via `gofiber/contrib/v3/i18n`

## Project Structure

```
asset-management/
├── internal/
│   ├── config/          # Viper-based config (package config)
│   ├── database/        # GORM database init + auto-migration (package database)
│   ├── handlers/        # HTTP handlers + App struct + authRequired middleware
│   │   ├── handler.go   # App struct definition
│   │   ├── auth.go      # Login / 2FA / logout handlers
│   │   ├── middleware.go # authRequired Fiber middleware
│   │   ├── categories.go # Category Master CRUD
│   │   ├── assets.go    # Asset Master CRUD
│   │   └── users.go     # User Master CRUD (Swagger-annotated)
│   ├── models/          # GORM model types: Category, Asset, User, Session
│   └── totp/            # RFC 6238 TOTP helpers
├── routes/
│   └── routes.go        # Centralized route registration
├── docs/                # swag-generated Swagger spec
├── localize/            # i18n YAML files (en.yaml, id.yaml)
├── templates/           # Gentelella Bootstrap HTML templates
├── main.go              # Thin entry point — wires everything together
└── main_test.go         # fiber.Test() integration tests (in-memory SQLite)
```

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

## Configuration

Configuration is resolved in priority order (highest first):

1. **Environment variables** — e.g. `export ADMIN_PASSWORD=secret`
2. **`config.yaml`** in the working directory (optional)
3. **Built-in defaults**

Example `config.yaml`:

```yaml
admin_username: admin
admin_password: admin123
totp_secret: JBSWY3DPEHPK3PXP
db_driver: sqlite
db_dsn: asset_management.db
port: "8080"
prefork: true
```

### All configuration keys

| Env / YAML key | Default | Description |
|---|---|---|
| `ADMIN_USERNAME` / `admin_username` | `admin` | Admin login username |
| `ADMIN_PASSWORD` / `admin_password` | `admin123` | Admin login password (bcrypt-hashed at startup) |
| `TOTP_SECRET` / `totp_secret` | `JBSWY3DPEHPK3PXP` | Base-32 TOTP secret |
| `DB_DRIVER` / `db_driver` | `sqlite` | Database driver (`sqlite` or `mysql`) |
| `DB_DSN` / `db_dsn` | `asset_management.db` | Data source name |
| `PORT` / `port` | `8080` | HTTP listen port |
| `PREFORK` / `prefork` | `true` | Enable Fiber Prefork |

## Swagger API Docs

Visit `http://localhost:8080/swagger/` after starting the server.

To regenerate after adding new API annotations:

```bash
swag init --parseDependency=true --dir ./,./internal/handlers --output ./docs
```

## i18n

Language is selected from the `Accept-Language` request header or the `?lang=` query parameter.

Supported: `en` (English, default) and `id` (Bahasa Indonesia).

Locale files live in `./localize/`. Add a new `<lang>.yaml` and register the `language.Tag` in `main.go` to extend.

## MySQL / MariaDB

Set these environment variables (or add to `config.yaml`) before starting:

```bash
export DB_DRIVER=mysql
export DB_DSN="user:password@tcp(host:3306)/asset_management?charset=utf8mb4&parseTime=True&loc=Local"
```

Create the database first: `CREATE DATABASE asset_management CHARACTER SET utf8mb4;`

