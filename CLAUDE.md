# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Multi-user blog platform. Go backend (Gin + GORM + PostgreSQL + Redis) serving a zero-build frontend (plain HTML/CSS/ES Modules). See `ServerRPD.md` (backend design) and `WebPRD.md` (frontend design) for full specs.

## Common Commands

All backend commands run from the `Server/` directory:

```bash
cd Server

# Development
make run                    # go run ./cmd/server
make deps                   # go mod tidy
make build                  # go build -o bin/server ./cmd/server

# Testing
make test                   # go test ./... -race -count=1
make integration            # go test -tags=integration ./test/integration/... -race -count=1 -timeout 5m
go test ./path -run TestName -v   # Run a single test

# Linting
make lint                   # golangci-lint run ./...

# Database migrations
make migrate-up             # Apply migrations/*.up.sql via psql

# Dependencies (Docker)
make docker-up              # docker compose up -d (MySQL + Redis)
make docker-down            # docker compose down
```

## Backend Architecture

### Dependency Injection
All wiring is manual in `cmd/server/main.go`. There is no DI framework. Components are instantiated bottom-up: repo -> service -> handler -> router.

### Layering
```
handler     (HTTP request/response, Gin context)
service     (business logic, validation)
repository  (GORM queries)
model       (GORM structs)
```

### Key Packages
- `internal/apperr` — Domain errors with codes and HTTP status mapping. All service errors return `*apperr.AppErr`. Handler uses `httpresp.Fail()` to convert to JSON.
- `internal/middleware` — Session store (Redis-backed), CSRF guard, rate limiting, recovery, request logging.
- `internal/pkg/httpresp` — Unified JSON envelope `{code, msg, data}`.
- `internal/worker` — Background goroutine that flushes Redis view counters to PostgreSQL on a ticker.

### Session & Auth
- Redis stores session data keyed by `sess:<sid>` and `csrf:<sid>`.
- Two cookies: `sid` (HttpOnly) and `csrf_token` (frontend-readable).
- `WithSession` middleware attaches session to Gin context and triggers sliding expiration (Touch + re-set cookies).
- `RequireAuth` rejects with code `2001` (frontend auto-redirects to login).
- `CSRFGuard` validates `X-CSRF-Token` header against Redis; failure returns code `2030` (frontend auto-refreshes page).

### Routing Groups
```
Public:  GET  /api/v1/articles, /api/v1/articles/:id, /api/v1/tags, /api/v1/users/:id/articles
         POST /api/v1/auth/register, /api/v1/auth/login
Private: POST /api/v1/articles, /api/v1/uploads/image
         PUT  /api/v1/articles/:id
         DELETE /api/v1/articles/:id
```
Private routes require `RequireAuth` + `CSRFGuard` + per-user rate limiting.

### Database
- **PostgreSQL** (not MySQL). `internal/db/mysql.go` retains its filename for historical reasons but uses `gorm.io/driver/postgres`.
- Auto-migration runs on startup (`gdb.AutoMigrate`).
- `docker-compose.yml` only provides MySQL + Redis. For local dev, start PostgreSQL separately or use an existing instance (e.g. Postgres.app on macOS).

### Configuration
`Server/config.yaml`. Override path with `BLOG_CONFIG` env var. Key fields:
- `server.static_dir: "../Web"` — Points to frontend files relative to `Server/`.
- `mysql.dsn` — Actually a PostgreSQL DSN (host, port, user, password, dbname, sslmode).

## Frontend Architecture

Zero-build. HTML pages are served directly by Gin's `StaticFile`/`Static`. Each page loads one `assets/js/pages/*.js` module.

### Shared Modules
- `assets/js/api.js` — `fetch` wrapper with CSRF header, unified error handling. Code `2001` -> redirect login, `2030` -> reload page.
- `assets/js/auth.js` — `getCurrentUser()` probes `/api/v1/auth/me` and caches result. `requireLogin()` redirects if unauthenticated.
- `assets/js/markdown.js` — Renders Markdown via `marked.parse`, strips `<script>` and `on*` attributes, then runs `hljs.highlightElement`.

### Components
- `navbar.js` — Injects nav into `<div id="navbar-mount">`. Renders auth-aware buttons.
- `pager.js` — Renders pagination into `<div id="pager-mount">`.
- `toast.js` — Appends toast messages to a fixed-position container.

## Testing

### Unit Tests
Standard Go tests alongside source files (`*_test.go`). Run with `make test`.

### Integration Tests
In `Server/test/integration/e2e_test.go`. Uses `testcontainers-go` to spin up MySQL and Redis containers. Marked with `//go:build integration`. Requires Docker. Run with `make integration`.

## Important Notes

- The `docker-compose.yml` in `Server/` launches MySQL (port 3306) and Redis (port 6379), but the application connects to PostgreSQL (port 5432). Ensure PostgreSQL is available separately.
- Frontend static files are served from `../Web` relative to the `Server/` working directory. If the binary is moved, `static_dir` in config must be updated.
- Uploads are stored in `Server/uploads/` and served at `/uploads/`.
