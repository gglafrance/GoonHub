# Contributing Guide

## Prerequisites

- Go 1.24+ (toolchain 1.24.12)
- Bun (for frontend package management)
- Docker & Docker Compose (for PostgreSQL)
- ffmpeg/ffprobe on PATH (for video processing)
- Air (optional, for Go hot reload)

## Environment Setup

### 1. Start PostgreSQL

```bash
cd docker && docker compose up -d
```

Verify with `docker compose ps` -- container should be `healthy`.

Connection details:
| Key      | Value                |
|----------|----------------------|
| Host     | localhost            |
| Port     | 5432                 |
| User     | goonhub              |
| Password | goonhub_dev_password |
| Database | goonhub              |

### 2. Configuration

Configuration is managed via YAML (not .env files). The development config is `config-dev.yaml` at the project root. Set `GOONHUB_CONFIG=config-dev.yaml` when running the backend.

Any config key can be overridden with environment variables using `GOONHUB_` prefix (dots become underscores):
```bash
export GOONHUB_SERVER_PORT=9090
export GOONHUB_DATABASE_HOST=remote-host
```

Key configuration sections:

| Section      | Purpose                                        |
|--------------|------------------------------------------------|
| `server`     | Port, timeouts                                 |
| `database`   | PostgreSQL connection and pooling              |
| `log`        | Level (`debug`/`info`/`warn`/`error`), format  |
| `auth`       | PASETO secret (32 bytes), admin creds, tokens  |
| `processing` | Frame intervals, quality, workers, directories |

### 3. Install Frontend Dependencies

```bash
cd web && bun install
```

### 4. Run Development Servers

Run both concurrently in separate terminals:

**Backend (port 8080):**
```bash
GOONHUB_CONFIG=config-dev.yaml air
# or: GOONHUB_CONFIG=config-dev.yaml go run ./cmd/server
```

**Frontend (port 3000):**
```bash
cd web && bun run dev
```

The Nuxt dev server proxies `/api`, `/thumbnails`, `/sprites`, `/vtt` to the backend on `:8080`.

---

## Available Scripts

### Frontend (web/package.json)

| Script        | Command          | Description                                      |
|---------------|------------------|--------------------------------------------------|
| `dev`         | `nuxt dev`       | Start Nuxt dev server on port 3000               |
| `build`       | `nuxt build`     | Production build (output to `web/dist`)          |
| `generate`    | `nuxt generate`  | Static site generation                           |
| `preview`     | `nuxt preview`   | Preview production build locally                 |
| `postinstall` | `nuxt prepare`   | Generate Nuxt types (runs after `bun install`)   |

### Backend (Makefile)

| Target       | Description                                           |
|--------------|-------------------------------------------------------|
| `make mocks` | Regenerate mock implementations for all repositories  |
| `make test`  | Regenerate mocks + run all tests                      |
| `make test-race` | Regenerate mocks + run tests with race detector  |
| `make test-cover` | Run tests with coverage report                  |

### Backend (Manual)

| Command | Description |
|---------|-------------|
| `go run github.com/google/wire/cmd/wire ./internal/wire` | Regenerate Wire DI |
| `go build -o goonhub ./cmd/server` | Build production binary |

---

## Development Workflow

### Making Backend Changes

1. Write your code changes
2. If you modified repository interfaces in `internal/data/`, run `make mocks`
3. If you modified DI providers in `internal/wire/wire.go`, run Wire: `go run github.com/google/wire/cmd/wire ./internal/wire`
4. Run tests: `make test-race`
5. Verify no regressions before committing

### Making Frontend Changes

1. Ensure dev server is running (`cd web && bun run dev`)
2. Changes hot-reload automatically
3. For production verification: `bun run build && bun run preview`

### Adding a New API Endpoint

1. Define handler in `internal/api/v1/handler/`
2. Register route in `internal/api/router.go`
3. Add request/response DTOs if needed (`internal/api/v1/request/`, `internal/api/v1/response/`)
4. Add corresponding tests
5. Run `make test-race`

### Adding a New Repository Method

1. Add method to interface in `internal/data/`
2. Implement in the corresponding repository file
3. Run `make mocks` to regenerate mock implementations
4. Write tests using the new mock
5. Run `make test-race`

---

## Testing

### Running Tests

```bash
# All tests
make test

# With race detector (required before merge)
make test-race

# With coverage
make test-cover
```

### Running Specific Package Tests

```bash
go test -v ./internal/core/
go test -v ./internal/jobs/
go test -v ./pkg/ffmpeg/
go test -v ./internal/api/middleware/
go test -v ./internal/api/v1/handler/
```

### Testing Conventions

- Use standard library `testing` only (no testify)
- Assertions via `t.Fatalf`/`t.Fatal`
- Table-driven tests for validation boundaries
- Concurrency tests must pass with `-race` flag
- Use `t.TempDir()` for file-based tests
- Use `gin.SetMode(gin.TestMode)` in handler tests
- Use `zap.NewNop()` for logger dependencies

---

## Database

### Migrations

Migrations run automatically on startup. They live in embedded files managed by `golang-migrate`.

### Reset Database

```bash
cd docker && docker compose down -v && docker compose up -d
```

### Connect via psql

```bash
docker exec -it goonhub-postgres psql -U goonhub -d goonhub
```

---

## Production Build

1. Build frontend: `cd web && bun run build`
2. Build Go binary (embeds frontend): `go build -o goonhub ./cmd/server`
3. The resulting `goonhub` binary is self-contained (frontend embedded via `embed.FS`)

---

## Code Style

- Go: constructor injection, wrap errors with context, snake_case JSON keys
- Vue/Nuxt: component decomposition at ~150 lines, Pinia stores, composables for shared logic
- No emoji in logs or comments
- No hardcoded values (use config)
- No unbounded goroutines (use worker pool)
