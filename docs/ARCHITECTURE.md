# Backend Architecture

This document describes the internal architecture of the GoonHub Go backend.

## System Overview

GoonHub is a single-binary application. The Nuxt frontend is compiled to static assets and embedded into the Go binary via `go:embed`. At runtime, the only external dependency is PostgreSQL.

```
                ┌──────────────────────────────────────────┐
                │              Go Binary                   │
                │  ┌──────────┐  ┌──────────────────────┐  │
                │  │ Embedded │  │     Gin HTTP Server  │  │
User ─── HTTP ──┼▶│ Frontend │  │  ┌────────────────┐  │  │
                │  │ (SPA)    │  │  │ API Handlers   │  │  │
                │  └──────────┘  │  └───────┬────────┘  │  │
                │                │          │           │  │
                │                │  ┌───────▼────────┐  │  │
                │                │  │ Core Services  │  │  │
                │                │  └───────┬────────┘  │  │
                │                │          │           │  │
                │                │  ┌───────▼────────┐  │  │
                │                │  │ Repositories   │  │  │
                │                │  └───────┬────────┘  │  │
                │                └──────────┼───────────┘  │
                └───────────────────────────┼──────────────┘
                                            │
                                    ┌───────▼────────┐
                                    │  PostgreSQL    │
                                    └────────────────┘
```

## Directory Structure

```
cmd/server/main.go                          Entry point
internal/
├── api/
│   ├── middleware/                          Auth, CORS, rate limiting, logging
│   ├── v1/handler/                         HTTP request handlers
│   ├── v1/request/                         Request DTOs
│   ├── v1/response/                        Response DTOs
│   ├── router.go                           Gin engine setup, static asset serving
│   └── routes.go                           Route registration
├── config/                                 Viper config loading
├── core/                                   Business logic services
├── data/                                   Models, repository interfaces, implementations
├── infrastructure/
│   ├── logging/                            Zap structured logger
│   ├── persistence/
│   │   ├── migrator/                       golang-migrate integration
│   │   │   └── migrations/                 SQL migration files (000001-000010)
│   │   └── postgres/                       GORM PostgreSQL connection
│   └── server/                             HTTP server lifecycle
├── jobs/                                   Worker pool, job types
├── mocks/                                  Generated mock implementations
├── pkg/context/                            Context utilities
├── pkg/errors/                             Error types
└── wire/                                   Google Wire DI configuration
pkg/
├── ffmpeg/                                 FFmpeg/FFprobe wrapper
└── scraper/                                Video metadata scraper
web.go                                      embed.FS directive for web/dist
```

## Dependency Injection

The application uses [Google Wire](https://github.com/google/wire) for compile-time dependency injection.

**Entry point:** `cmd/server/main.go` calls `wire.InitializeServer(cfgPath)`.

**Wire configuration:** `internal/wire/wire.go` defines all providers in the `wire.Build(...)` call. The provider order follows the dependency graph:

1. **Config** - `config.Load` reads YAML + env vars
2. **Infrastructure** - Logger, PostgreSQL connection
3. **Repositories** - All data access implementations (Video, User, RBAC, JobHistory, PoolConfig, ProcessingConfig, TriggerConfig)
4. **Core Services** - EventBus, JobHistoryService, VideoProcessingService, VideoService, AuthService, UserService, SettingsService, RBACService, AdminService, TriggerScheduler
5. **Middleware** - IP rate limiter
6. **Handlers** - Video, Auth, Settings, Admin, Job, SSE
7. **Router** - Gin engine with middleware and routes
8. **Server** - HTTP server wrapping the router

**Adding a new component:**

1. Define the interface/struct in the appropriate package
2. Create a `provideXxx` function in `wire.go`
3. Add it to the `wire.Build(...)` call
4. Run `go run github.com/google/wire/cmd/wire ./internal/wire`

## Request Lifecycle

### Middleware Chain

Every request passes through middleware in this order:

```
Request
  │
  ├─ gin.Recovery()          Panic recovery
  ├─ RequestID()             Generates UUID, sets X-Request-ID header
  ├─ Logger()                Structured request logging (method, path, latency, status)
  ├─ CORS                    Origin validation, method/header allowlisting
  │
  ├─ [Public routes]         No further middleware
  │
  ├─ AuthMiddleware()        Validates Bearer token (PASETO), sets user in context
  │   ├─ RequireRole()       Checks user.Role == required role (admin routes)
  │   └─ RequirePermission() Checks RBAC cache for role:permission mapping
  │
  └─ Handler                 Processes request, returns JSON response
```

### Handler Pattern

Handlers receive a `*gin.Context`, extract/validate request data, call the appropriate service, and return JSON:

```go
func (h *VideoHandler) GetVideo(c *gin.Context) {
    id := c.Param("id")
    video, err := h.service.GetByID(parsedID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "..."})
        return
    }
    c.JSON(http.StatusOK, video)
}
```

## Data Layer

### Repository Pattern

All data access is defined by interfaces in `internal/data/`:

- `VideoRepository` - Video CRUD, processing status updates, phase queries
- `UserRepository` - User CRUD, password/role updates
- `RevokedTokenRepository` - Token revocation checks
- `UserSettingsRepository` - Player/app preferences (Upsert)
- `RoleRepository` - Role listing with preloaded permissions
- `PermissionRepository` - Permission sync for roles
- `JobHistoryRepository` - Job tracking, cleanup by age
- `PoolConfigRepository` - Worker pool sizes (Upsert)
- `ProcessingConfigRepository` - Quality settings (Upsert)
- `TriggerConfigRepository` - Cron/trigger definitions (Upsert)

Implementations use GORM and live alongside the interfaces. Configuration repositories use **Upsert semantics** (insert or update on conflict).

### Models

Core models defined in `internal/data/models.go`:

| Model | Key Fields |
|-------|-----------|
| `User` | username, password (bcrypt), role, last_login_at |
| `Video` | title, stored_path, duration, dimensions, processing_status, thumbnail/sprite paths, metadata (codecs, bitrate, fps) |
| `Role` | name, description, permissions (M2M) |
| `Permission` | name, description |
| `RevokedToken` | token_hash (SHA-256), expires_at, reason |
| `UserSettings` | autoplay, volume, loop, videos_per_page, sort_order |
| `JobHistory` | job_id, video_id, phase, status, error_message, timestamps |

### Migrations

Schema evolution is managed by [golang-migrate](https://github.com/golang-migrate/migrate). Migration files are embedded in the binary and run automatically on startup.

Files live in `internal/infrastructure/persistence/migrator/migrations/`:

| Migration | Purpose |
|-----------|---------|
| 000001 | Initial schema (users, videos, revoked_tokens) |
| 000002 | User settings table |
| 000003 | RBAC tables (roles, permissions, role_permissions) |
| 000004 | Job history tracking |
| 000005 | Pool config table |
| 000006 | Multi-resolution thumbnail columns |
| 000007 | Extended video metadata (fps, bitrate, codecs) |
| 000008 | Processing config table |
| 000009 | Sprites concurrency setting |
| 000010 | Trigger config table |

## Video Processing Pipeline

Videos are processed asynchronously through three sequential phases using dedicated worker pools.

```
Upload ──▶ Save File ──▶ Create DB Record ──▶ Publish Event
                                                    │
                                                    ▼
                                         SubmitVideo() [async]
                                                    │
                              ┌──────────────────── │ ────────────────────┐
                              │                     ▼                     │
                              │           ┌─────────────────┐             │
                              │           │  Metadata Pool  │             │
                              │           │  (3 workers)    │             │
                              │           └────────┬────────┘             │
                              │                    │                      │
                              │         Extract: duration, resolution,    │
                              │         fps, bitrate, codecs              │
                              │                    │                      │
                              │         ┌──────────┴──────────┐           │
                              │         ▼                     ▼           │
                              │  ┌──────────────┐   ┌──────────────┐      │
                              │  │ Thumbnail    │   │ Sprites      │      │
                              │  │ Pool         │   │ Pool         │      │
                              │  │ (1 worker)   │   │ (1 worker)   │      │
                              │  └──────┬───────┘   └──────┬───────┘      │
                              │         │                  │              │
                              │    sm + lg thumbs    sprite sheets + VTT  │
                              │         │                  │              │
                              │         └────────┬─────────┘              │
                              │                  ▼                        │
                              │         Mark video "completed"            │
                              │         Publish video:completed event     │
                              └───────────────────────────────────────────┘
```

### Phase Details

**Metadata** (`MetadataJob`):
- Runs `ffprobe` to extract duration, resolution, frame rate, bitrate, codecs
- Calculates tile dimensions for small (320px) and large (1280px) thumbnails
- Updates the Video record with extracted metadata

**Thumbnail** (`ThumbnailJob`):
- Seeks to configured position (default `00:00:05`) or percentage of duration
- Extracts a single frame at two resolutions (small + large)
- Encodes as WebP at configured quality levels
- Output: `{id}_thumb_sm.webp`, `{id}_thumb_lg.webp`

**Sprites** (`SpritesJob`):
- Extracts frames at configured interval (default every 5 seconds)
- Arranges frames into a grid (default 12x8 = 96 per sheet)
- Generates WebVTT file with timestamp-to-sprite-coordinate mappings
- Supports configurable concurrency for parallel ffmpeg extraction
- Output: `{id}_sprites.webp`, `{id}_thumbnails.vtt`

### Trigger System

Phase execution can be controlled by trigger configurations:

| Trigger Type | Behavior |
|-------------|----------|
| `on_import` | Phase runs immediately when a video is uploaded |
| `after_job` | Phase runs after another phase completes (configurable dependency) |
| `scheduled` | Phase runs on a cron schedule for videos needing it |
| `manual` | Phase only runs when explicitly triggered via admin API |

Cycle detection prevents circular `after_job` dependencies.

## Worker Pool Architecture

Each processing phase has its own `WorkerPool` instance (`internal/jobs/worker_pool.go`).

```go
type WorkerPool struct {
    workerCount int              // Goroutine count
    jobQueue    chan Job          // Buffered (capacity 100)
    resultChan  chan JobResult    // Buffered (capacity 100)
    ctx/cancel  context.Context  // Lifecycle control
    running     atomic.Bool      // State flag
}
```

**Key behaviors:**
- `Submit(job)` is non-blocking (sends to buffered channel). Returns error if pool is stopped.
- Workers read from `jobQueue`, execute the job, and write results to `resultChan`.
- A dedicated goroutine per pool reads from `resultChan` and dispatches to phase-completion handlers.
- `Stop()` cancels context, closes `jobQueue`, waits for all workers via WaitGroup, then closes `resultChan`.

**Dynamic resizing:** Pool worker counts can be changed at runtime via the admin API. This creates a new pool with the desired size, starts it, swaps it in, and stops the old pool.

## Authentication & Authorization

### PASETO Tokens

Authentication uses [PASETO v2](https://paseto.io/) symmetric encryption (not JWT):

1. **Login:** Validate credentials (bcrypt) → Generate PASETO token with user payload → Return token
2. **Validate:** Check revocation (SHA-256 hash lookup) → Decrypt token → Verify expiry → Return `UserPayload`
3. **Revoke:** On logout, hash the token and store it in `revoked_tokens` with the original expiry time

The PASETO symmetric key must be exactly 32 bytes. In development, a random key is auto-generated per startup.

### RBAC

Role-Based Access Control is implemented via the `RBACService`:

- Roles and permissions are stored in the database (M2M relationship via `role_permissions`)
- On startup, all role-permission mappings are loaded into an in-memory cache (`map[string]map[string]bool`)
- `HasPermission(role, permission)` checks the cache (no DB query per request)
- Admin API operations that modify roles trigger `Sync()` to refresh the cache
- Permissions are string-based (e.g., `videos:upload`, `videos:view`, `videos:delete`, `videos:reprocess`)

## Real-Time Updates (SSE)

### EventBus

The `EventBus` (`internal/core/event_bus.go`) is a simple pub/sub system:

```go
type EventBus struct {
    subscribers map[string]chan VideoEvent  // Buffered channels (cap 50)
}
```

- **Publish:** Iterates all subscribers, sends event via non-blocking select. If a subscriber's channel is full, the event is dropped (logged as warning).
- **Subscribe:** Returns a unique ID and a read-only channel.
- **Unsubscribe:** Closes and removes the channel.

### SSE Handler

The SSE endpoint (`GET /api/v1/events?token=<token>`) authenticates via query parameter (not the Authorization header, since EventSource API doesn't support custom headers):

1. Validate token from query param
2. Subscribe to EventBus
3. Stream events as `data: {json}\n\n`
4. Send keepalive comments every 30 seconds
5. Unsubscribe on client disconnect

## Trigger Scheduler

The `TriggerScheduler` (`internal/core/trigger_scheduler.go`) uses [robfig/cron/v3](https://github.com/robfig/cron) to run scheduled processing phases:

- On startup, loads all `scheduled` type triggers from the database
- Registers cron entries that query for videos needing the phase and submit them for processing
- `RefreshSchedules()` removes all existing entries and reloads from DB (called when admin updates trigger config)
- Supports standard 5-field cron expressions (minute, hour, day-of-month, month, day-of-week)

## Configuration System

Configuration is loaded via [Viper](https://github.com/spf13/viper) with three layers (lowest to highest priority):

1. **Defaults** in code (e.g., port 8080, 3 metadata workers)
2. **YAML file** specified by `GOONHUB_CONFIG` environment variable
3. **Environment variables** prefixed with `GOONHUB_` (dots become underscores)

Example: `processing.frame_interval` can be overridden by `GOONHUB_PROCESSING_FRAME_INTERVAL`.

### Runtime Configuration

Some settings are stored in the database and can be changed at runtime without restart:

- **Pool config** - Worker counts per phase (1-10 range, validated)
- **Processing config** - Thumbnail dimensions, quality levels, sprites concurrency
- **Trigger config** - Phase trigger types and cron expressions

On startup, DB values override YAML defaults. Changes via admin API take effect immediately (worker pools are resized, cron schedules are refreshed).

## Server Lifecycle

### Startup Sequence

```
main.go
  │
  ├─ wire.InitializeServer(cfgPath)
  │   ├─ Load config (YAML + env vars)
  │   ├─ Initialize logger (Zap)
  │   ├─ Connect to PostgreSQL (GORM)
  │   ├─ Run migrations (golang-migrate, embedded)
  │   ├─ Create repository implementations
  │   ├─ Create core services
  │   ├─ Create handlers
  │   ├─ Build Gin router
  │   └─ Return *Server
  │
  └─ server.Start()
      ├─ EnsureAdminExists() - Creates admin user if not present
      ├─ processingService.Start() - Launches 3 worker pools + result processors
      ├─ jobHistoryService.StartCleanupTicker() - Periodic old record removal
      ├─ triggerScheduler.Start() - Loads and registers cron schedules
      ├─ ListenAndServe() on configured port
      └─ Wait for SIGINT/SIGTERM
```

### Graceful Shutdown

On receiving SIGINT or SIGTERM:

1. Create a 5-second timeout context
2. Call `http.Server.Shutdown(ctx)` - stops accepting new connections, waits for active requests
3. Deferred cleanup runs in reverse order:
   - `triggerScheduler.Stop()` - Stops cron, waits for running jobs
   - `jobHistoryService.StopCleanupTicker()` - Stops periodic cleanup
   - `processingService.Stop()` - Cancels worker contexts, drains queues, waits for workers

## Static Asset Serving

Generated assets (thumbnails, sprites, VTT files) are served directly by the Gin router from the **configured directories** (set via `processing.thumbnail_dir`, `processing.sprite_dir`, `processing.vtt_dir` in config). Default paths: `./data/metadata/thumbnails`, `./data/metadata/sprites`, `./data/metadata/vtt`.

| Path | Content | Cache |
|------|---------|-------|
| `GET /thumbnails/:id?size=sm\|lg` | WebP thumbnail | 1 year |
| `GET /sprites/:filename` | WebP sprite sheet | 1 year |
| `GET /vtt/:videoId` | WebVTT file | 1 year |

The embedded frontend SPA is served via a `NoRoute` handler with fallback to `index.html` for client-side routing.

## Logging

Structured logging via [Zap](https://github.com/uber-go/zap):

- **Development:** Console format with colors (cyan=INFO, yellow=WARN, red=ERROR)
- **Production:** JSON format for log aggregation
- Components tag their logs (e.g., `component=worker_pool`, `pool=metadata`)
- Request logging includes method, path, status, latency, IP, request ID
