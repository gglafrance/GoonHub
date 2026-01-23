# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Goonhub is a self-hosted video library application with a Go backend and Nuxt 3 (Vue 3) frontend. Videos are uploaded, processed (thumbnails, sprite sheets, VTT files generated via ffmpeg), and streamed. The frontend is embedded into the Go binary for single-binary production deployment.

## Development Commands

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
- `internal/data/` - GORM models and repository interfaces/implementations (SQLite)
- `internal/infrastructure/` - Server, logging (zap), SQLite persistence
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

### API Routes

All under `/api/v1/`:

- `POST /auth/login` (public, rate-limited)
- `GET /auth/me`, `POST /auth/logout` (authenticated)
- `POST /videos`, `GET /videos`, `GET /videos/:id`, `DELETE /videos/:id`, `GET /videos/:id/reprocess` (authenticated)
- `GET /videos/:id/stream` (public)

### Configuration

Config loaded via Viper: YAML file path set by `GOONHUB_CONFIG` env var. All config keys can be overridden with `GOONHUB_` prefixed env vars (dots become underscores, e.g. `GOONHUB_SERVER_PORT`).

### External Dependencies

- **ffmpeg/ffprobe** must be available on PATH for video processing
- **SQLite** is the database (file: `library.db` by default)

## Coding Conventions

### Go Backend

- Never ignore errors; wrap with context: `fmt.Errorf("failed to do X: %w", err)`
- Use constructor injection; register new components in `internal/wire/wire.go`
- Do not hardcode values; add them to `internal/config/` structs
- Use Worker Pool pattern for concurrency (no unbounded goroutines)
- All API responses are JSON with `snake_case` keys; Go structs use PascalCase

### Frontend Aesthetics

The UI follows a **Cinematic, Immersive Media Aesthetic**:

- OLED-friendly dark mode: absolute black (`#000000`) or deep charcoal (`#0F0F0F`) backgrounds
- Neon accents: Electric Green (`#2ECC71`) for actions/progress, Hot Red (`#FF4757`) for primary CTAs
- Glassmorphism: `backdrop-filter: blur()` for floating elements (modals, toasts, cards)
- Generous border-radius (12-24px), spacious layout, media-dominant hierarchy
- Avoid: low-contrast grays, sharp edges, heavy borders, generic Bootstrap blue

### Prohibited

- No emoji in log messages or code comments
- No data deletion without explicit confirmation
