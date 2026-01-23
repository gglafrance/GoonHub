# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Goonhub is a self-hosted video library application with a Go backend and Nuxt 3 (Vue 3) frontend. Videos are uploaded, processed (thumbnails, sprite sheets, VTT files generated via ffmpeg), and streamed. The frontend is embedded into the Go binary for single-binary production deployment.

## Development Commands

### Database (PostgreSQL)

```bash
# Start PostgreSQL via Docker
cd docker && docker compose up -d

# Verify healthy
docker compose ps

# Connect via psql
docker exec -it goonhub-postgres psql -U goonhub -d goonhub

# Reset database (destroys all data)
cd docker && docker compose down -v && docker compose up -d
```

### Backend (Go)

```bash
# Hot reload with Air (preferred for dev)
GOONHUB_CONFIG=config-dev.yaml air

# Or run directly
GOONHUB_CONFIG=config-dev.yaml go run ./cmd/server

# Regenerate Wire dependency injection (after changing providers in wire.go)
wire ./internal/wire/

# Build production binary (requires frontend build first)
go build -o goonhub ./cmd/server
```

### Frontend (Nuxt 3 / Vue 3)

```bash
cd web

# Install dependencies
bun install

# Dev server on :3000 (proxies /api, /thumbnails, /sprites, /vtt to backend on :8080)
bun run dev

# Production build (output to web/dist, embedded into Go binary)
bun run build
```

### Full Stack Development

Run the Go backend on port 8080 and Nuxt dev server on port 3000 simultaneously. The Nuxt dev server proxies API routes to the backend.

## Architecture

### Backend Structure

- `cmd/server/main.go` - Entry point, initializes via Wire DI
- `internal/wire/` - Google Wire dependency injection (run `wire ./internal/wire/` after changing providers)
- `internal/config/` - Viper-based config, loaded from YAML file or `GOONHUB_*` env vars
- `internal/api/` - Gin HTTP router, routes, middleware (CORS, auth, rate limiting)
- `internal/api/v1/handler/` - Request handlers (video, auth)
- `internal/core/` - Business logic services (VideoService, VideoProcessingService, AuthService, UserService)
- `internal/data/` - GORM models and repository interfaces/implementations
- `internal/infrastructure/` - Server, logging (zap), PostgreSQL persistence
- `internal/infrastructure/persistence/postgres/` - GORM PostgreSQL initializer with connection pooling
- `internal/infrastructure/persistence/migrator/` - golang-migrate based schema migrations
- `internal/jobs/` - Worker pool and video processing jobs
- `pkg/ffmpeg/` - ffmpeg wrapper for metadata extraction, thumbnails, sprite sheets, VTT generation
- `web.go` - `embed.FS` directive embedding `web/dist` into the binary

### Frontend Structure (web/app/)

- Nuxt 4 directory structure with `app/` subdirectory
- `pages/` - Routes: index (video grid), login, watch/[id]
- `components/` - VideoCard, VideoPlayer, etc.
- `stores/` - Pinia stores (auth with sessionStorage persistence, videos)
- `composables/` - Reusable composition functions
- `types/` - TypeScript interfaces (Video, Auth)
- `assets/css/main.css` - Tailwind CSS 4 entry point

### Key Patterns

- **DI**: Google Wire generates `wire_gen.go`; edit `wire.go` then regenerate
- **Auth**: PASETO tokens, admin user auto-created on startup, token revocation via DB
- **Video Processing Pipeline**: Upload -> save file -> create DB record -> submit async job (worker pool) -> extract metadata -> generate thumbnail -> generate sprite sheets -> generate VTT -> update DB
- **Static Assets**: Thumbnails, sprites, VTT files served from `./data/` directory
- **Frontend Proxy**: In dev, Vite proxies `/api`, `/thumbnails`, `/sprites`, `/vtt` to `:8080`
- **Custom Elements**: Vue compiler configured to treat `media-*`, `videojs-video`, `media-theme` as custom elements
- **Auto Imports**: Pinia stores and composables auto-imported via Nuxt config

### API Routes

All under `/api/v1/`:

- `POST /auth/login` (public, rate-limited)
- `GET /auth/me`, `POST /auth/logout` (authenticated)
- `POST /videos`, `GET /videos`, `GET /videos/:id`, `DELETE /videos/:id`, `GET /videos/:id/reprocess` (authenticated)
- `GET /videos/:id/stream` (public)

### Configuration

Config loaded via Viper: YAML file path set by `GOONHUB_CONFIG` env var. All config keys can be overridden with `GOONHUB_` prefixed env vars (dots become underscores, e.g. `GOONHUB_SERVER_PORT`).

### Database

- **PostgreSQL 18** is the database (run via `docker/docker-compose.yml`)
- Migrations are managed by `golang-migrate` (embedded in binary, run automatically on startup)

### External Dependencies

- **ffmpeg/ffprobe** must be available on PATH for video processing
- **PostgreSQL 18** via Docker (see `docker/` directory)

## Coding Conventions

### Go Backend

- Never ignore errors; wrap with context: `fmt.Errorf("failed to do X: %w", err)`
- Use constructor injection; register new components in `internal/wire/wire.go`
- Do not hardcode values; add them to `internal/config/` structs
- Use Worker Pool pattern for concurrency (no unbounded goroutines)
- All API responses are JSON with `snake_case` keys; Go structs use PascalCase

### Frontend Aesthetics

The UI follows a **Deep Space SaaS Aesthetic**—sophisticated, dark, and highly technical:

- **Color & Theme:** Strict **Deep Dark Mode** with deep midnight/void black backgrounds (`#050505` to `#0F0F0F`). Use subtle white borders (10-15% opacity) to define panels rather than shadows.
    - **Accents:** Sharp, glowing **Lava Red/Coral** (`#FF4D4D`) for primary highlights, active tabs, and gradients
    - **Status Colors:** Vibrant emerald green for toggle switches and "active" states
- **Layout & Structure:** Dense, information-rich layouts with clear hierarchy. Floating elements use backdrop-filter: blur() with low opacity backgrounds to create depth
- **Typography:** Technical, geometric sans-serif (Inter, Geist, SF Pro). High hierarchy: bright white headers, muted grey (60%) secondary text. Relatively small font sizes (12-14px) for information density
- **Interaction:** Immediate, subtle hover effects (lighten or 1px border glow). Snappy toggle transitions. Pill-shaped or rounded rectangle inputs with subtle inner glows
- **Texture & Depth:** Use borders and subtle gradients to separate layers. Input fields and cards should feel crisp and defined against the deep background
- Avoid: Light themes, high-brightness backgrounds, purple/blue AI gradients (stick to Red/Black/White palette), large chunky padding, flat design without depth

### Prohibited

- No emoji in log messages or code comments
- No data deletion without explicit confirmation
