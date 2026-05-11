# asset-management

Assets Management by ABB Insurance Brokers.

## Features
- Login with bcrypt password verification
- Two-Factor Authentication (TOTP 6-digit code)
- Category master CRUD
- Asset master CRUD (linked to category master)
- Metis-style admin layout

## Run
```bash
go mod tidy
go run .
```

Open `http://localhost:8080`
- Default username: `admin`
- Default password: `admin123`
- Default TOTP secret: `JBSWY3DPEHPK3PXP`

Use environment variables to override defaults:
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `TOTP_SECRET`
