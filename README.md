# GoonHub

Privacy-first local NSFW video library manager.

## Development

### Backend with Hot Reload (Air)
```bash
air
```

### Frontend Development
```bash
cd web && npm run dev
```

### Standard Backend Run (no hot reload)
```bash
go run cmd/server/main.go
```

### Production Build
```bash
cd web && npm run generate
cd ..
go build -o goonhub ./cmd/server
./goonhub
```

## Testing

The Go backend has a test suite covering services, middleware, handlers, worker pool, and VTT generation. Tests use mocked repository interfaces and require no external dependencies (no database, no ffmpeg).

```bash
# Run all tests (regenerates mocks first)
make test

# Run with race detector (recommended for concurrency tests)
make test-race

# Run with coverage report
make test-cover

# Regenerate mocks after changing repository interfaces
make mocks
```

## Project Structure

- `cmd/server/` - Application entry point
- `internal/` - Private application code (api, core, data, jobs)
- `internal/mocks/` - Generated mock implementations for repository interfaces
- `pkg/` - Public libraries (ffmpeg, scraper)
- `web/` - Vue 3 + Nuxt 3 frontend
