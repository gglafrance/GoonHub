# Backend Developer Guide

This guide covers setting up the development environment, daily workflow, and common tasks for working on the GoonHub Go backend.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.24+ | Backend language |
| PostgreSQL | 18 | Database (via Docker) |
| Docker + Compose | Latest | PostgreSQL container |
| ffmpeg / ffprobe | Latest | Video processing |
| Bun | Latest | Frontend package manager (for full-stack dev) |
| Air | Latest | Go hot-reload (`go install github.com/air-verse/air@latest`) |

## Environment Setup

### 1. Start PostgreSQL

```bash
cd docker && docker compose up -d
```

Verify it's healthy:

```bash
docker compose ps
# Should show goonhub-postgres as "healthy"
```

Connection details (from `docker-compose.yml`):
- Host: `localhost:5432`
- User: `goonhub`
- Password: `goonhub_dev_password`
- Database: `goonhub`

### 2. Configuration

The development config file is `config-dev.yaml` at the project root. The backend reads it via the `GOONHUB_CONFIG` environment variable:

```bash
export GOONHUB_CONFIG=config-dev.yaml
```

Key development defaults:
- Server port: `8080`
- Log format: `console` (colored output)
- PASETO secret: hardcoded 32-byte string (tokens persist across restarts)
- Admin credentials: `admin` / `admin`

### 3. Run the Backend

Using Air (hot-reload on file changes):

```bash
GOONHUB_CONFIG=config-dev.yaml air
```

Or directly:

```bash
GOONHUB_CONFIG=config-dev.yaml go run ./cmd/server
```

The server starts on `:8080`. Migrations run automatically on startup.

### 4. Run the Frontend (optional)

For full-stack development, run the Nuxt dev server alongside the backend:

```bash
cd web && bun install && bun run dev
```

The frontend runs on `:3000` and proxies `/api`, `/thumbnails`, `/sprites`, `/vtt` to the backend on `:8080`.

## Development Workflow

### Hot Reload

Air watches for `.go` file changes and restarts the server. Configuration is in `.air.toml` (if present) or uses defaults.

### Verify Changes

After any backend code change:

```bash
make test-race
```

This regenerates mocks and runs all tests with the race detector enabled.

### Database Reset

To start fresh (destroys all data):

```bash
cd docker && docker compose down -v && docker compose up -d
```

### Connect to Database

```bash
docker exec -it goonhub-postgres psql -U goonhub -d goonhub
```

## Configuration Reference

All values can be set in the YAML file or overridden with `GOONHUB_` prefixed environment variables.

### Server

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `8080` | HTTP listen port |
| `server.read_timeout` | `15s` | Max time to read request |
| `server.write_timeout` | `15s` | Max time to write response |
| `server.idle_timeout` | `60s` | Keep-alive timeout |
| `server.allowed_origins` | `["http://localhost:3000"]` | CORS allowed origins |

### Database

| Key | Default | Description |
|-----|---------|-------------|
| `database.host` | `localhost` | PostgreSQL host |
| `database.port` | `5432` | PostgreSQL port |
| `database.user` | `goonhub` | Database user |
| `database.password` | `goonhub_dev_password` | Database password |
| `database.dbname` | `goonhub` | Database name |
| `database.sslmode` | `disable` | SSL mode |
| `database.max_open_conns` | `25` | Max open connections |
| `database.max_idle_conns` | `5` | Max idle connections |

### Logging

| Key | Default | Description |
|-----|---------|-------------|
| `log.level` | `info` | Log level (debug, info, warn, error) |
| `log.format` | `console` | Output format (console, json) |

### Processing

| Key | Default | Description |
|-----|---------|-------------|
| `processing.frame_interval` | `5` | Seconds between sprite frames |
| `processing.max_frame_dimension` | `320` | Small thumbnail longest side (px) |
| `processing.max_frame_dimension_large` | `1280` | Large thumbnail longest side (px) |
| `processing.frame_quality` | `85` | Small thumbnail WebP quality (1-100) |
| `processing.frame_quality_lg` | `85` | Large thumbnail WebP quality (1-100) |
| `processing.frame_quality_sprites` | `75` | Sprite sheet WebP quality (1-100) |
| `processing.metadata_workers` | `3` | Concurrent metadata extraction jobs |
| `processing.thumbnail_workers` | `1` | Concurrent thumbnail generation jobs |
| `processing.sprites_workers` | `1` | Concurrent sprite sheet generation jobs |
| `processing.thumbnail_seek` | `00:00:05` | Timestamp for thumbnail extraction |
| `processing.frame_output_dir` | `./data/frames` | Frame extraction output directory |
| `processing.thumbnail_dir` | `./data/thumbnails` | Thumbnail output directory |
| `processing.sprite_dir` | `./data/sprites` | Sprite sheet output directory |
| `processing.vtt_dir` | `./data/vtt` | VTT file output directory |
| `processing.grid_cols` | `12` | Sprite sheet grid columns |
| `processing.grid_rows` | `8` | Sprite sheet grid rows |
| `processing.sprites_concurrency` | `0` | Parallel ffmpeg processes for sprites (0=auto) |
| `processing.job_history_retention` | `7d` | How long to keep job history records |

### Auth

| Key | Default | Description |
|-----|---------|-------------|
| `auth.paseto_secret` | (empty) | 32-byte hex string for token encryption |
| `auth.admin_username` | `admin` | Auto-created admin username |
| `auth.admin_password` | `admin` | Auto-created admin password |
| `auth.token_duration` | `24h` | Token validity period |
| `auth.login_rate_limit` | `10` | Login attempts per minute per IP |
| `auth.login_rate_burst` | `5` | Burst size for rate limiter |

## Adding a New Feature

This walkthrough covers adding a new entity from database to API endpoint.

### 1. Define the Model

Add a GORM model to `internal/data/models.go`:

```go
type Widget struct {
    ID        uint      `gorm:"primarykey" json:"id"`
    CreatedAt time.Time `json:"created_at"`
    Name      string    `gorm:"not null" json:"name"`
}
```

### 2. Create a Migration

Create a new numbered migration file:

```
internal/infrastructure/persistence/migrator/migrations/000011_widgets.up.sql
```

```sql
CREATE TABLE widgets (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name TEXT NOT NULL
);
```

Also create the down migration:

```
internal/infrastructure/persistence/migrator/migrations/000011_widgets.down.sql
```

```sql
DROP TABLE IF EXISTS widgets;
```

### 3. Define the Repository Interface

Add to `internal/data/repository.go` (or a new file):

```go
type WidgetRepository interface {
    Create(widget *Widget) error
    GetByID(id uint) (*Widget, error)
    List() ([]Widget, error)
}
```

### 4. Implement the Repository

Create `internal/data/widget_repository.go`:

```go
type widgetRepositoryImpl struct {
    db *gorm.DB
}

func NewWidgetRepository(db *gorm.DB) WidgetRepository {
    return &widgetRepositoryImpl{db: db}
}

func (r *widgetRepositoryImpl) Create(widget *Widget) error {
    return r.db.Create(widget).Error
}
// ... etc
```

### 5. Create the Service

Create `internal/core/widget_service.go` with business logic. Accept the repository via constructor injection:

```go
type WidgetService struct {
    repo   data.WidgetRepository
    logger *zap.Logger
}

func NewWidgetService(repo data.WidgetRepository, logger *zap.Logger) *WidgetService {
    return &WidgetService{repo: repo, logger: logger}
}
```

### 6. Create the Handler

Create `internal/api/v1/handler/widget.go`:

```go
type WidgetHandler struct {
    service *core.WidgetService
}

func NewWidgetHandler(service *core.WidgetService) *WidgetHandler {
    return &WidgetHandler{service: service}
}

func (h *WidgetHandler) List(c *gin.Context) {
    widgets, err := h.service.List()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list widgets"})
        return
    }
    c.JSON(http.StatusOK, widgets)
}
```

### 7. Register Routes

Add routes in `internal/api/routes.go`:

```go
widgets := protected.Group("/widgets")
{
    widgets.GET("", widgetHandler.List)
    widgets.POST("", widgetHandler.Create)
}
```

### 8. Wire It Up

Add provider functions to `internal/wire/wire.go`:

```go
func provideWidgetRepository(db *gorm.DB) data.WidgetRepository {
    return data.NewWidgetRepository(db)
}

func provideWidgetService(repo data.WidgetRepository, logger *logging.Logger) *core.WidgetService {
    return core.NewWidgetService(repo, logger.Logger)
}

func provideWidgetHandler(service *core.WidgetService) *handler.WidgetHandler {
    return handler.NewWidgetHandler(service)
}
```

Add them to the `wire.Build(...)` call, and add the handler parameter to `provideRouter` and `RegisterRoutes`.

### 9. Regenerate Wire

```bash
go run github.com/google/wire/cmd/wire ./internal/wire
```

### 10. Generate Mocks and Test

```bash
# Add mockgen line to Makefile, then:
make mocks
make test-race
```

## Database Migrations

### Creating Migrations

Migration files use the naming convention `{number}_{description}.{up|down}.sql`. Numbers are zero-padded to 6 digits and must be sequential.

```bash
# Next migration number (check existing files first)
ls internal/infrastructure/persistence/migrator/migrations/

# Create both up and down files
touch internal/infrastructure/persistence/migrator/migrations/000011_add_widgets.up.sql
touch internal/infrastructure/persistence/migrator/migrations/000011_add_widgets.down.sql
```

Migrations are embedded in the binary via `go:embed` and run automatically on startup. There is no CLI tool for manual migration management - to rollback, connect to the database directly and run the down migration SQL.

### Migration Tips

- Always create both up and down files
- Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
- Add indexes for frequently queried columns
- Use `TIMESTAMPTZ` for timestamp columns
- PostgreSQL arrays use the `text[]` type (mapped to `pq.StringArray` in Go)

## Testing

### Running Tests

```bash
make test           # Regenerate mocks + run all tests
make test-race      # With race detector (mandatory before committing)
make test-cover     # Generate coverage report

# Run specific package
go test -v ./internal/core/
go test -v ./internal/api/v1/handler/
go test -v ./internal/jobs/
go test -v ./pkg/ffmpeg/
```

### Writing Tests

**Conventions:**
- Use standard `testing` package only (no testify)
- Assertions via `t.Fatalf` / `t.Fatal` / `t.Errorf`
- Mock repositories with `go.uber.org/mock` (mocks in `internal/mocks/`)
- Table-driven tests for validation boundaries
- Same-package (white-box) tests for services
- Use `gin.SetMode(gin.TestMode)` in handler tests
- Use `zap.NewNop()` for logger dependencies
- Use `t.TempDir()` for file-based tests

**Example service test:**

```go
func TestWidgetService_Create(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := mocks.NewMockWidgetRepository(ctrl)
    logger := zap.NewNop()
    svc := NewWidgetService(mockRepo, logger)

    mockRepo.EXPECT().Create(gomock.Any()).Return(nil)

    err := svc.Create("test-widget")
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
}
```

**Example table-driven test:**

```go
func TestWidgetService_Validate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid name", "my-widget", false},
        {"empty name", "", true},
        {"too long", strings.Repeat("x", 256), true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("validateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
            }
        })
    }
}
```

### Mock Generation

When you add or modify a repository interface:

1. Update the interface in `internal/data/`
2. Add a `mockgen` line to the `Makefile` if it's a new interface
3. Run `make mocks`

Mock files are generated to `internal/mocks/mock_{name}_repository.go`.

## Debugging

### Log Levels

Set `log.level` in config or via `GOONHUB_LOG_LEVEL` environment variable:

| Level | Use Case |
|-------|----------|
| `debug` | Worker pool operations, event publishing, job queue depth |
| `info` | Request logging, job start/complete, pool sizing |
| `warn` | Dropped SSE events (subscriber buffer full) |
| `error` | Job failures, DB errors, processing errors |

### Common Issues

**"PASETO key must be 32 bytes":**
The `auth.paseto_secret` config value must be exactly 32 bytes (64 hex characters). In development, leave it empty or set a 32-character ASCII string.

**"worker pool is stopped":**
A job was submitted after the pool was stopped (usually during shutdown). This is expected during graceful shutdown.

**Missing ffmpeg:**
Ensure `ffmpeg` and `ffprobe` are on your PATH. Video processing jobs will fail without them.

**Port already in use:**
Another instance is running on port 8080. Kill it or change `server.port` in config.

**Migration errors:**
Migrations are forward-only. If you need to undo a migration, connect to the database and run the `.down.sql` file manually, then update the `schema_migrations` table.

## Building for Production

### 1. Build the Frontend

```bash
cd web && bun install && bun run build
```

This outputs to `web/dist/`, which is embedded into the Go binary.

### 2. Build the Binary

```bash
go build -o goonhub ./cmd/server
```

The resulting `goonhub` binary is self-contained (frontend + backend + migrations).

### 3. Run in Production

```bash
GOONHUB_CONFIG=/etc/goonhub/config.yaml \
GOONHUB_AUTH_PASETO_SECRET=<64-hex-chars> \
GOONHUB_ENVIRONMENT=production \
./goonhub
```

Required for production:
- `GOONHUB_AUTH_PASETO_SECRET` must be set (not auto-generated)
- `GOONHUB_ENVIRONMENT=production` enables Gin release mode
- PostgreSQL must be accessible
- `ffmpeg` / `ffprobe` must be on PATH
- The `./data/` directory must be writable (thumbnails, sprites, VTT, uploaded videos)
