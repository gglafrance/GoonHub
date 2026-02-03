# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Goonhub is a self-hosted video library application with a Go backend and Nuxt 4 (Vue 3) frontend. Scenes are uploaded, processed (thumbnails, sprite sheets, VTT files generated via ffmpeg), and streamed. The frontend is embedded into the Go binary for single-binary production deployment. Features include RBAC, real-time updates via SSE, dynamic worker pool configuration, scheduled job triggers, and Meilisearch-powered full-text search.

## Development Commands

### Database & Search (PostgreSQL + Meilisearch)

```bash
# Start PostgreSQL and Meilisearch via Docker
cd docker && docker compose up -d

# Verify healthy
docker compose ps

# Connect via psql
docker exec -it goonhub-postgres psql -U goonhub -d goonhub
```

### Backend (Go)

```bash
# Run directly
GOONHUB_CONFIG=config-dev.yaml go run ./cmd/server

# Regenerate Wire dependency injection (after changing providers in wire.go)
go run github.com/google/wire/cmd/wire ./internal/wire

# Build production binary (requires frontend build first)
go build -o goonhub ./cmd/server
```

### Frontend (Nuxt 4 / Vue 3)

```bash
cd web

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
- `internal/api/v1/handler/` - Request handlers (scene, auth, admin, settings, SSE, job history)
- `internal/api/v1/request/` - Request DTOs (admin, auth, settings)
- `internal/api/v1/response/` - Response DTOs and envelopes (auth, pagination, error responses)
- `internal/core/` - Business logic services:
    - `scene_service.go` - Scene CRUD operations
    - `scene_processing_service.go` - Processing orchestration and queue status
    - `search_service.go` - Search orchestration using Meilisearch
    - `auth_service.go` - PASETO token management
    - `user_service.go` - User management
    - `admin_service.go` - Admin operations
    - `rbac_service.go` - Role-Based Access Control
    - `settings_service.go` - User settings management
    - `job_history_service.go` - Job history, retention, and pending job management
    - `job_status_service.go` - Aggregated job status for real-time header display
    - `job_queue_feeder.go` - DB-backed queue feeder that polls pending jobs and submits to worker pools
    - `event_bus.go` - EventBus for real-time SSE event publishing
    - `trigger_scheduler.go` - Cron-based scheduled triggers for processing phases
- `internal/data/` - GORM models and repository interfaces/implementations:
    - `scene_models.go` - Scene, SceneTag, SceneActor models
    - `models.go` - User, Role, Permission, RolePermission, RevokedToken, UserSettings
    - `job_models.go` - JobHistory (with Priority field), DLQEntry, RetryConfigRecord, job status constants (`JobStatusPending`, `JobStatusRunning`, etc.)
    - `repository.go` - Core repository interfaces (SceneRepository, UserRepository, RevokedTokenRepository, UserSettingsRepository)
    - `rbac_repository.go` - RBACRepository for roles/permissions
    - `job_history_repository.go` - JobHistoryRepository (includes `ClaimPendingJobs`, `CountPendingByPhase`, `ExistsPendingOrRunning` for DB-backed queue)
    - `pool_config_repository.go` - PoolConfigRepository (dynamic worker pool settings)
    - `processing_config_repository.go` - ProcessingConfigRepository (quality/concurrency settings)
    - `trigger_config_repository.go` - TriggerConfigRepository (cron/after_job/on_import triggers)
- `internal/infrastructure/` - Server, logging (zap), PostgreSQL persistence, Meilisearch client
- `internal/infrastructure/meilisearch/` - Meilisearch client wrapper (indexing, search, health checks)
- `internal/infrastructure/persistence/postgres/` - GORM PostgreSQL initializer with connection pooling
- `internal/infrastructure/persistence/migrator/` - golang-migrate based schema migrations (35 migrations)
- `internal/jobs/` - Worker pool and scene processing jobs (metadata, thumbnail, sprites)
- `internal/apperrors/` - Typed error system with domain-specific errors:
    - `errors.go` - Base AppError interface and common error types (NotFoundError, ValidationError, ConflictError, InternalError, ForbiddenError, UnauthorizedError)
    - `scene.go` - Scene-specific errors (ErrSceneNotFound, ErrInvalidFileExtension, ErrSceneDimensionsNotAvailable)
    - `auth.go` - Auth-specific errors (ErrInvalidCredentials, ErrTokenExpired, ErrInsufficientPermissions)
    - `codes.go` - Error code constants for API responses
- `internal/storage/` - Storage abstraction layer:
    - `storage.go` - Storage interface for file operations (Save, Read, Delete, Exists, MkdirAll, Stat, Glob, Join)
    - `local.go` - LocalStorage implementation using local filesystem
- `internal/lifecycle/` - Lifecycle management for graceful shutdown:
    - `manager.go` - LifecycleManager with Go() for tracked goroutines and Shutdown() for coordinated cleanup
- `internal/mocks/` - Generated mock implementations for all repository interfaces (via `go.uber.org/mock`)
- `pkg/ffmpeg/` - ffmpeg wrapper for metadata extraction, thumbnails, sprite sheets, VTT generation
- `web.go` - `embed.FS` directive embedding `web/dist` into the binary

### Frontend Structure (web/app/)

- Nuxt 4 directory structure with `app/` subdirectory
- `pages/` - Routes: index (scene grid), login, watch/[id], settings
- `components/` - Organized by feature:
    - Root: `AppHeader`, `SceneCard`, `SceneGrid`, `ScenePlayer`, `SceneUpload`, `SceneMetadata`, `UploadIndicator`, `Pagination`, `ErrorAlert`, `LoadingSpinner`
    - `header/` - `JobStatus` (real-time job indicator), `JobStatusPopup` (detailed breakdown by phase)
    - `settings/` - `Account`, `Player`, `App`, `Users`, `Jobs` (tab orchestrator)
    - `settings/jobs/` - `HistoryTab`, `QueueStatus`, `ActiveJobs`, `Workers`, `Processing`, `Triggers`, `Retry`, `DLQ`, `Manual`
    - `settings/` modals - `UserCreateModal`, `UserEditRoleModal`, `UserResetPasswordModal`, `UserDeleteModal`
    - `watch/` - `Details` (orchestrator), `DetailTabs`, `Jobs`, `Actors`
    - `watch/details/` - `TitleEditor`, `DescriptionEditor`, `ReleaseDateEditor`, `RatingPanel`, `InteractionsBar`, `TagManager`, `PornDBStatus`
    - `search/` - `SearchFilters` (orchestrator), `SearchBar`, `SearchResults`
    - `search/filters/` - `FilterSection`, `FilterTags`, `FilterActors`, `FilterDuration`, `FilterDateRange`, `FilterSelect`, `FilterLiked`, `FilterRatingRange`, `FilterJizzRange`
    - `ui/` - Reusable `ErrorAlert`, `LoadingSpinner`
- `stores/` - Pinia stores: `auth` (sessionStorage), `scenes`, `upload`, `settings`, `search`, `jobStatus` (real-time job counts via SSE)
- `composables/` - Organized by domain:
    - `api/` - Domain-specific API composables (see API Composables section below)
    - `useApi.ts` - Unified API facade re-exporting all domain composables (backwards-compatible)
    - `useSettingsMessage`, `useFormatter`, `useThumbnailPreview`, `useSSE`, `useVttParser`
    - `useSceneRating`, `useSceneLike`, `useSceneJizzCount` - Scene interaction state
    - `useWatchTracking`, `useResumePosition` - Scene player tracking
    - `useJobFormatting`, `useJobPagination`, `useJobAutoRefresh` - Job management utilities
    - `useInlineEditor` - Reusable inline editing pattern
- `types/` - TypeScript interfaces: `scene`, `auth`, `settings`, `admin`, `jobs`, `tag`
- `assets/css/main.css` - Tailwind CSS 4 entry point

### API Composables (web/app/composables/api/)

API functions are organized into domain-specific composables for better code organization:

- `useApiCore.ts` - Shared fetch helper with auth/error handling
- `useApiScenes.ts` - Scene CRUD, search, streaming, interactions, watch tracking
- `useApiSettings.ts` - User settings (player, app, tags, password)
- `useApiAdmin.ts` - Users, roles, permissions management
- `useApiJobs.ts` - Job history, pool config, processing config, triggers, retry config
- `useApiTags.ts` - Tag CRUD, scene-tag associations
- `useApiActors.ts` - Actor CRUD, associations, interactions
- `useApiPornDB.ts` - PornDB search, performers, scenes, metadata
- `useApiStorage.ts` - Storage paths, validation, scanning
- `useApiDLQ.ts` - Dead letter queue operations

For backwards compatibility, `useApi()` re-exports all functions from domain composables. New code should prefer importing domain-specific composables directly (e.g., `useApiScenes()` instead of `useApi()`).

### Key Patterns

- **DI**: Google Wire generates `wire_gen.go`; edit `wire.go` then regenerate
- **Auth**: PASETO tokens, admin user auto-created on startup, token revocation via DB
- **RBAC**: Roles and permissions managed via database, enforced by middleware
- **Scene Processing Pipeline**: Upload -> save file -> create DB record -> create pending job in DB -> JobQueueFeeder claims job -> worker pool executes -> extract metadata -> generate thumbnails (multi-resolution) -> generate sprite sheets -> generate VTT -> update DB
- **DB-Backed Job Queue**: Jobs are created with `status='pending'` in `job_history` table (non-blocking). `JobQueueFeeder` polls DB every 2 seconds, claims up to 50 pending jobs using `FOR UPDATE SKIP LOCKED`, and submits to worker pool channels (1000 capacity buffer). This pattern handles 80,000+ videos without blocking: DB acts as infinite overflow, channel acts as immediate buffer. Deduplication is enforced via unique index on `(scene_id, phase)` for active jobs. Orphaned jobs (running > 5 minutes on startup) are automatically recovered and marked failed for retry.
- **Real-Time Updates (SSE)**: EventBus publishes SceneEvents -> SSEHandler streams to connected clients via Server-Sent Events. Token auth via query parameter. 30-second keepalive pings. Buffered channel (50 events) prevents blocking.
- **Trigger Scheduler**: Cron-based scheduling via robfig/cron/v3. Supports trigger types: `on_import`, `after_job`, `manual`, `scheduled`. Includes cycle detection for after_job dependencies.
- **Dynamic Configuration**: Worker pool size, processing quality, and trigger schedules are stored in DB and configurable at runtime via admin API.
- **Queue Status Monitoring**: `SceneProcessingService.GetQueueStatus()` returns queued jobs per phase for frontend display.
- **Meilisearch Full-Text Search**: SearchService orchestrates search operations via Meilisearch. Meilisearch handles full-text search and attribute filtering (tags, actors, studio, duration, resolution, date). PostgreSQL handles user-specific filters (liked, rating, jizz_count) via pre-filtering scene IDs which are then passed to Meilisearch. Scenes are indexed on: upload, update, delete, tag changes, and metadata extraction completion.
- **Typed Error Handling**: Services return typed errors from `internal/apperrors/` package. Handlers use type-checking functions (`apperrors.IsNotFound()`, `apperrors.IsValidation()`) and `response.Error()` helper for consistent API error responses with proper HTTP status codes and error codes.
- **Response Envelopes**: Standardized API responses via `internal/api/v1/response/envelope.go`. Use `response.OK()`, `response.Created()`, `response.Error()` helpers. Paginated responses use `PaginatedResponse[T]` with `Pagination` metadata.
- **Lifecycle Management**: Use `internal/lifecycle/Manager` for tracked goroutines. Call `lifecycle.Go(name, fn)` instead of raw `go func()` to ensure graceful shutdown coordination.
- **Static Assets**: Thumbnails, sprites, VTT files served from configured directories (`processing.thumbnail_dir`, `processing.sprite_dir`, `processing.vtt_dir`). Defaults: `./data/thumbnails`, `./data/sprites`, `./data/vtt`
- **Frontend Proxy**: In dev, Vite proxies `/api`, `/thumbnails`, `/sprites`, `/vtt` to `:8080`
- **Custom Elements**: Vue compiler configured to treat `media-*`, `videojs-video`, `media-theme` as custom elements
- **Auto Imports**: Pinia stores and composables auto-imported via Nuxt config

### Configuration

Config loaded via Viper: YAML file path set by `GOONHUB_CONFIG` env var. All config keys can be overridden with `GOONHUB_` prefixed env vars (dots become underscores, e.g. `GOONHUB_SERVER_PORT`).

### Database

- **PostgreSQL 18** is the database (run via `docker/docker-compose.yml`)
- Migrations are managed by `golang-migrate` (embedded in binary, run automatically on startup)
- **IMPORTANT:** Before making any database schema changes or additions, read `docs/DATABASE.md` for the complete schema reference, naming conventions, and architectural patterns

### External Dependencies

- **ffmpeg/ffprobe** must be available on PATH for scene processing
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
- **Typed errors:** Return typed errors from `internal/apperrors/` instead of raw `fmt.Errorf()`. In services, wrap `gorm.ErrRecordNotFound` with `apperrors.NewNotFoundError()`. In handlers, use `apperrors.IsNotFound(err)` for type checking and `response.Error(c, err)` for consistent responses.
- **Response helpers:** Use `response.OK()`, `response.Created()`, `response.NoContent()`, `response.Error()` from `internal/api/v1/response/` instead of raw `c.JSON()` for consistent API responses.
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
- For typed error testing: mock repos should return `gorm.ErrRecordNotFound` for not-found cases; assert with `apperrors.IsNotFound(err)`

### Vue/Nuxt Conventions

- **Component decomposition:** When a component exceeds ~150 lines, extract logical sections into sub-components under `components/<feature>/<subfeature>/`. Nuxt auto-imports them as `<FeatureSubfeatureComponent />` (e.g., `components/watch/details/TitleEditor.vue` becomes `<WatchDetailsTitleEditor />`).
- **Orchestrator pattern:** Parent components should be thin orchestrators (~100-150 lines) that handle tab state, layout, and conditional rendering. They compose sub-components and pass minimal props. Examples: `Details.vue` orchestrates editor components, `Jobs.vue` orchestrates tab content, `SearchFilters.vue` orchestrates filter components.
- **Self-sufficient components:** Each sub-component manages its own state via stores and composables. Use `defineExpose()` to expose reload methods when parent needs to trigger refresh (e.g., after metadata update).
- **Composables for shared patterns:** Extract repeated ref+logic patterns into `composables/use*.ts`. Nuxt auto-imports them. Examples:
    - `useInlineEditor()` for title/description/date editing with auto-save
    - `useSceneRating()` for star rating with hover states
    - `useJobFormatting()` for duration/time/status formatting
    - `useJobPagination()` for pagination with localStorage persistence
- **API composables by domain:** Use domain-specific API composables (e.g., `useApiScenes()`, `useApiTags()`) instead of the unified `useApi()` for better tree-shaking and code organization.
- **Reusable filter pattern:** For collapsible filter sections, use the `FilterSection.vue` wrapper component that handles expand/collapse state and badge display.
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

### Project Memories

When working on GoonHub, remember:

- Always import icons using NuxtIcon from `@nuxt/icons` (no direct SVG imports)
- After any Go backend change, run `make test-race` before considering the task complete
- When adding new repository interface methods, regenerate mocks with `make mocks`
- PASETO key must be exactly 32 bytes for v2 symmetric encryption
- Admin routes require RBAC middleware with admin role check
- Use <NuxtTime :datetime=".." /> for date display
- API composables are in `composables/api/` - prefer domain-specific imports (e.g., `useApiScenes()`) over unified `useApi()`
- Component sub-directories follow pattern: `components/<feature>/<subfeature>/` auto-imports as `<FeatureSubfeatureComponent />`
- Use typed errors from `internal/apperrors/` - check with `apperrors.IsNotFound()`, `apperrors.IsValidation()`, etc.
- In handlers, use `response.Error(c, err)` for automatic HTTP status and error code mapping
- Never add a `Co-Authored-By` line to git commit messages
- Before any database schema change, read `docs/DATABASE.md` for schema reference and conventions
