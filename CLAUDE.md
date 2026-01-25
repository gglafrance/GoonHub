# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Goonhub is a self-hosted video library application with a Go backend and Nuxt 4 (Vue 3) frontend. Videos are uploaded, processed (thumbnails, sprite sheets, VTT files generated via ffmpeg), and streamed. The frontend is embedded into the Go binary for single-binary production deployment. Features include RBAC, real-time updates via SSE, dynamic worker pool configuration, scheduled job triggers, and Meilisearch-powered full-text search.

## Development Commands

### Database & Search (PostgreSQL + Meilisearch)

```bash
# Start PostgreSQL and Meilisearch via Docker
cd docker && docker compose up -d

# Verify healthy
docker compose ps

# Connect via psql
docker exec -it goonhub-postgres psql -U goonhub -d goonhub

# Reset database (destroys all data including search index)
cd docker && docker compose down -v && docker compose up -d

# Trigger full search reindex (via API, requires admin auth)
curl -X POST http://localhost:8080/api/v1/admin/search/reindex -H "Authorization: Bearer <token>"

# Check Meilisearch health
curl http://localhost:7700/health
```

### Backend (Go)

```bash
# Hot reload with Air (preferred for dev)
GOONHUB_CONFIG=config-dev.yaml air

# Or run directly
GOONHUB_CONFIG=config-dev.yaml go run ./cmd/server

# Regenerate Wire dependency injection (after changing providers in wire.go)
go run github.com/google/wire/cmd/wire ./internal/wire

# Build production binary (requires frontend build first)
go build -o goonhub ./cmd/server
```

### Frontend (Nuxt 4 / Vue 3)

```bash
cd web

# Install dependencies
bun install

# Dev server on :3000 (proxies /api, /thumbnails, /sprites, /vtt to backend on :8080)
bun run dev

# Production build (output to web/dist, embedded into Go binary)
bun run build
```

### Testing (Go Backend)

```bash
# Regenerate mocks (required after changing repository interfaces in internal/data/)
make mocks

# Run all tests
make test

# Run tests with race detector (critical for concurrency tests)
make test-race

# Run tests with coverage report
make test-cover

# Run specific package tests
go test -v ./internal/core/
go test -v ./internal/jobs/
go test -v ./pkg/ffmpeg/
go test -v ./internal/api/middleware/
go test -v ./internal/api/v1/handler/
```

### Full Stack Development

Run the Go backend on port 8080 and Nuxt dev server on port 3000 simultaneously. The Nuxt dev server proxies API routes to the backend.

## Architecture

### Backend Structure

- `cmd/server/main.go` - Entry point, initializes via Wire DI
- `internal/wire/` - Google Wire dependency injection (run `wire ./internal/wire/` after changing providers)
- `internal/config/` - Viper-based config, loaded from YAML file or `GOONHUB_*` env vars
- `internal/api/` - Gin HTTP router, routes, middleware (CORS, auth, rate limiting, RBAC)
- `internal/api/v1/handler/` - Request handlers (video, auth, admin, settings, SSE, job history)
- `internal/api/v1/request/` - Request DTOs (admin, auth, settings)
- `internal/api/v1/response/` - Response DTOs (auth)
- `internal/core/` - Business logic services:
    - `video_service.go` - Video CRUD operations
    - `video_processing_service.go` - Processing orchestration and queue status
    - `search_service.go` - Search orchestration using Meilisearch
    - `auth_service.go` - PASETO token management
    - `user_service.go` - User management
    - `admin_service.go` - Admin operations
    - `rbac_service.go` - Role-Based Access Control
    - `settings_service.go` - User settings management
    - `job_history_service.go` - Job history and retention
    - `event_bus.go` - EventBus for real-time SSE event publishing
    - `trigger_scheduler.go` - Cron-based scheduled triggers for processing phases
- `internal/data/` - GORM models and repository interfaces/implementations:
    - `models.go` - User, Video, Role, Permission, RolePermission, RevokedToken, UserSettings, JobHistory
    - `repository.go` - Core repository interfaces (VideoRepository, UserRepository, RevokedTokenRepository, UserSettingsRepository)
    - `rbac_repository.go` - RBACRepository for roles/permissions
    - `job_history_repository.go` - JobHistoryRepository
    - `pool_config_repository.go` - PoolConfigRepository (dynamic worker pool settings)
    - `processing_config_repository.go` - ProcessingConfigRepository (quality/concurrency settings)
    - `trigger_config_repository.go` - TriggerConfigRepository (cron/after_job/on_import triggers)
- `internal/infrastructure/` - Server, logging (zap), PostgreSQL persistence, Meilisearch client
- `internal/infrastructure/meilisearch/` - Meilisearch client wrapper (indexing, search, health checks)
- `internal/infrastructure/persistence/postgres/` - GORM PostgreSQL initializer with connection pooling
- `internal/infrastructure/persistence/migrator/` - golang-migrate based schema migrations (10 migrations)
- `internal/jobs/` - Worker pool and video processing jobs (metadata, thumbnail, sprites)
- `internal/mocks/` - Generated mock implementations for all repository interfaces (via `go.uber.org/mock`)
- `pkg/ffmpeg/` - ffmpeg wrapper for metadata extraction, thumbnails, sprite sheets, VTT generation
- `web.go` - `embed.FS` directive embedding `web/dist` into the binary

### Frontend Structure (web/app/)

- Nuxt 4 directory structure with `app/` subdirectory
- `pages/` - Routes: index (video grid), login, watch/[id], settings
- `components/` - Organized by feature:
    - Root: `AppHeader`, `VideoCard`, `VideoGrid`, `VideoPlayer`, `VideoUpload`, `VideoMetadata`, `UploadIndicator`, `Pagination`, `ErrorAlert`, `LoadingSpinner`
    - `settings/` - `Account`, `Player`, `App`, `Users`, `Jobs` (with sub-components: `jobs/Workers`, `jobs/Processing`, `jobs/Triggers`)
    - `settings/` modals - `UserCreateModal`, `UserEditRoleModal`, `UserResetPasswordModal`, `UserDeleteModal`
    - `watch/` - `DetailTabs`, `Jobs`
    - `ui/` - Reusable `ErrorAlert`, `LoadingSpinner`
- `stores/` - Pinia stores: `auth` (sessionStorage), `videos`, `upload`, `settings`
- `composables/` - `useApi`, `useSettingsMessage`, `useFormatter`, `useThumbnailPreview`, `useSSE`, `useVttParser`
- `types/` - TypeScript interfaces: `video`, `auth`, `settings`, `admin`, `jobs`
- `assets/css/main.css` - Tailwind CSS 4 entry point

### Key Patterns

- **DI**: Google Wire generates `wire_gen.go`; edit `wire.go` then regenerate
- **Auth**: PASETO tokens, admin user auto-created on startup, token revocation via DB
- **RBAC**: Roles and permissions managed via database, enforced by middleware
- **Video Processing Pipeline**: Upload -> save file -> create DB record -> submit async job (worker pool) -> extract metadata -> generate thumbnails (multi-resolution) -> generate sprite sheets -> generate VTT -> update DB
- **Real-Time Updates (SSE)**: EventBus publishes VideoEvents -> SSEHandler streams to connected clients via Server-Sent Events. Token auth via query parameter. 30-second keepalive pings. Buffered channel (50 events) prevents blocking.
- **Trigger Scheduler**: Cron-based scheduling via robfig/cron/v3. Supports trigger types: `on_import`, `after_job`, `manual`, `scheduled`. Includes cycle detection for after_job dependencies.
- **Dynamic Configuration**: Worker pool size, processing quality, and trigger schedules are stored in DB and configurable at runtime via admin API.
- **Queue Status Monitoring**: `VideoProcessingService.GetQueueStatus()` returns queued jobs per phase for frontend display.
- **Meilisearch Full-Text Search**: SearchService orchestrates search operations via Meilisearch. Meilisearch handles full-text search and attribute filtering (tags, actors, studio, duration, resolution, date). PostgreSQL handles user-specific filters (liked, rating, jizz_count) via pre-filtering video IDs which are then passed to Meilisearch. Videos are indexed on: upload, update, delete, tag changes, and metadata extraction completion.
- **Static Assets**: Thumbnails, sprites, VTT files served from `./data/` directory
- **Frontend Proxy**: In dev, Vite proxies `/api`, `/thumbnails`, `/sprites`, `/vtt` to `:8080`
- **Custom Elements**: Vue compiler configured to treat `media-*`, `videojs-video`, `media-theme` as custom elements
- **Auto Imports**: Pinia stores and composables auto-imported via Nuxt config

### API Routes

All under `/api/v1/`:

**Public:**

- `POST /auth/login` (rate-limited)
- `GET /videos/:id/stream`
- `GET /events?token=<token>` (SSE real-time event stream)

**Authenticated:**

- `GET /auth/me`, `POST /auth/logout`
- `POST /videos`, `GET /videos`, `GET /videos/:id`, `DELETE /videos/:id`
- `GET /settings`, `PUT /settings`

**Admin (requires admin role):**

- `GET /admin/jobs` - List job history
- `GET /admin/pool-config`, `PUT /admin/pool-config` - Worker pool configuration
- `GET /admin/processing-config`, `PUT /admin/processing-config` - Processing quality settings
- `GET /admin/trigger-config`, `PUT /admin/trigger-config` - Trigger schedule management
- `POST /admin/videos/:id/process/:phase` - Manually trigger processing phase
- `GET /admin/search/status` - Check Meilisearch availability
- `POST /admin/search/reindex` - Trigger full search index rebuild

### Configuration

Config loaded via Viper: YAML file path set by `GOONHUB_CONFIG` env var. All config keys can be overridden with `GOONHUB_` prefixed env vars (dots become underscores, e.g. `GOONHUB_SERVER_PORT`).

Key config sections:

- `server` - Port, timeouts
- `database` - PostgreSQL connection, pooling
- `log` - Level, format
- `auth` - PASETO secret, admin credentials, token duration, rate limits
- `processing` - Frame intervals, dimensions (small/large), quality levels (thumbnails/sprites), worker counts per phase, sprite grid size, concurrency, output directories, job history retention
- `meilisearch` - Host, API key, index name (required for search functionality)

### Database

- **PostgreSQL 18** is the database (run via `docker/docker-compose.yml`)
- Migrations are managed by `golang-migrate` (embedded in binary, run automatically on startup)
- 10 migration files covering: initial schema, user settings, RBAC, job history, pool config, multi-resolution thumbnails, extended metadata, processing config, sprites concurrency, trigger config

### External Dependencies

- **ffmpeg/ffprobe** must be available on PATH for video processing
- **PostgreSQL 18** via Docker (see `docker/` directory)
- **Meilisearch v1.33** via Docker (required for search functionality)

### Tech Stack Versions

- Go 1.24 (toolchain 1.24.12)
- Nuxt 4.2 / Vue 3.5
- Gin 1.11, GORM 1.31
- Pinia 3.0, Tailwind CSS 4.1
- video.js 8.23, media-chrome 4.17
- Meilisearch v1.33, meilisearch-go v0.27.0

## Coding Conventions

### Go Backend

- Never ignore errors; wrap with context: `fmt.Errorf("failed to do X: %w", err)`
- Use constructor injection; register new components in `internal/wire/wire.go`
- Do not hardcode values; add them to `internal/config/` structs
- Use Worker Pool pattern for concurrency (no unbounded goroutines)
- All API responses are JSON with `snake_case` keys; Go structs use PascalCase
- **Testing is mandatory:** After modifying Go backend code, always run `make test-race` to verify no regressions or data races. When adding new service methods or handlers, add corresponding tests in the same package.

### Go Testing Conventions

- Standard library `testing` only (no testify) — use `t.Fatalf`/`t.Fatal` for assertions
- Mock repository interfaces via `go.uber.org/mock` — mocks live in `internal/mocks/` and are generated with `make mocks`
- Use table-driven tests for validation boundaries (volumes, sort orders, file extensions)
- Concurrency tests must pass with `-race` flag
- Use `t.TempDir()` for file-based tests (auto-cleaned)
- Tests are same-package (white-box) for services, allowing access to unexported fields
- Use `gin.SetMode(gin.TestMode)` in handler/middleware tests
- Use `zap.NewNop()` for logger dependencies in tests

### Vue/Nuxt Conventions

- **Component decomposition:** When a page exceeds ~150 lines, extract logical sections into sub-components under `components/<page-name>/`. Nuxt auto-imports them as `<PageNameComponent />` (e.g., `components/settings/Account.vue` becomes `<SettingsAccount />`).
- **Self-sufficient components:** Each sub-component manages its own state via stores and composables. Parent pages are thin orchestrators (tab state, layout, conditional rendering) that pass no props to tab-level children.
- **Composables for shared patterns:** Extract repeated ref+logic patterns into `composables/use*.ts`. Nuxt auto-imports them. Example: `useSettingsMessage()` for message/error state.
- **Modal pattern:** Modals receive `visible` + entity props and emit `close` + success events (`created`, `updated`, `deleted`). Modals own their form/loading state and display errors internally. Always wrap with `<Teleport to="body">`.
- **No manual imports for auto-imported APIs:** Never import `ref`, `computed`, `watch`, `onMounted` from Vue, or stores/composables — Nuxt auto-imports them. Only use explicit `import type` for TypeScript types from `~/types/`.
- **Data loading with `v-if` tabs:** Components rendered with `v-if` mount fresh each time the tab activates — use `onMounted` to load data (no watcher needed on the parent).
- **Watchers for store sync:** When form fields mirror store state, use `watch(() => store.state, syncFn)` + `onMounted(syncFn)` to keep local refs in sync.

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

### Project Memories (keep updating this section as you come across important patterns/observations)

When working on GoonHub, remember:

- Always import icons using NuxtIcon from `@nuxt/icons` (no direct SVG imports)
- After any Go backend change, run `make test-race` before considering the task complete
- When adding new repository interface methods, regenerate mocks with `make mocks`
- PASETO key must be exactly 32 bytes for v2 symmetric encryption
- Worker pool's `Submit()` returns an error if the pool is stopped (not a panic)
- EventBus uses a buffered channel (cap 50); slow subscribers may miss events
- TriggerScheduler supports cycle detection for `after_job` trigger chains
- SSE endpoint authenticates via `?token=` query param (not Authorization header)
- Configuration repositories (pool, processing, trigger) all use Upsert semantics
- Admin routes require RBAC middleware with admin role check
- Settings sub-components under `components/settings/jobs/` are nested one level deeper (e.g., `<SettingsJobsWorkers />`)
- SearchService uses `VideoIndexer` interface for indexing hooks - services call `SetIndexer()` during server startup
- Meilisearch index is configured with searchable (title, filename, description, actors, tag_names), filterable (studio, actors, tag_ids, duration, height, created_at, processing_status, id), and sortable (created_at, title, duration) attributes
- User-specific filters (liked, rating, jizz_count) are handled by querying PostgreSQL for matching video IDs first, then passing those IDs as a filter to Meilisearch
- Meilisearch is required for search functionality - there is no PostgreSQL fallback
- Use <NuxtTime :datetime=".." /> for date display
