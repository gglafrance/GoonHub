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

## Project Structure

- `cmd/server/` - Application entry point
- `internal/` - Private application code (api, core, data, jobs)
- `pkg/` - Public libraries (ffmpeg, scraper)
- `web/` - Vue 3 + Nuxt 3 frontend
