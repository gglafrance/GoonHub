# Docker Infrastructure

## Quick Start

### Development (build locally)

```bash
cd docker
docker compose up -d --build
```

This builds the application from source and starts all services with development settings.

For hot-reloading during development, run only the dependencies:
```bash
cd docker
docker compose up -d postgres meilisearch
```
Then run the backend and frontend locally (see main CLAUDE.md for commands).

### Production (pre-built images)

```bash
cd docker

# 1. Copy and customize configuration
cp config.yaml config.prod.yaml
# Edit config.prod.yaml as needed

# 2. Set up secrets
cp .env.example .env
# Edit .env with secure passwords

# 3. Start services
docker compose -f docker-compose.prod.yml up -d
```

## Configuration

### Development

Development uses `config-dev.yaml` which has sensible defaults for local development. No configuration needed.

### Production

Configuration is split into two files:

| File | Purpose |
|------|---------|
| `config.prod.yaml` | Application settings (non-sensitive) |
| `.env` | Secrets only (passwords, API keys) |

**config.prod.yaml** - Edit for:
- Server timeouts and CORS settings
- Log level and format
- Processing workers and quality settings
- Token duration

**`.env`** - Required secrets:

| Variable | Description |
|----------|-------------|
| `GOONHUB_DATABASE_PASSWORD` | PostgreSQL password |
| `GOONHUB_AUTH_PASETO_SECRET` | 32-byte token signing secret |
| `GOONHUB_AUTH_ADMIN_PASSWORD` | Admin account password |
| `MEILI_MASTER_KEY` | Meilisearch API key |

Secrets in `.env` override values in `config.prod.yaml`.

## Services

| Service | Port | Description |
|---------|------|-------------|
| app | 8080 | GoonHub application |
| postgres | 5432 | PostgreSQL database |
| meilisearch | 7700 | Full-text search engine |

## Connection Details (Development)

### PostgreSQL

| Field | Value |
|-------|-------|
| Host | localhost |
| Port | 5432 |
| User | goonhub |
| Password | goonhub_dev_password |
| Database | goonhub |

```bash
docker exec -it goonhub-postgres psql -U goonhub -d goonhub
```

### Meilisearch

| Field | Value |
|-------|-------|
| Host | http://localhost:7700 |
| Master Key | goonhub_dev_master_key |

## Database Management

### Reset database
```bash
docker compose down -v
docker compose up -d
```

### Backup
```bash
docker exec goonhub-postgres pg_dump -U goonhub goonhub > backup.sql
```

### Restore
```bash
docker exec -i goonhub-postgres psql -U goonhub -d goonhub < backup.sql
```

## Building the Docker Image

The multi-stage Dockerfile builds:
1. Frontend (Bun + Nuxt)
2. Backend (Go with embedded frontend)
3. Runtime (Alpine + ffmpeg)

Manual build from repo root:
```bash
docker build -f docker/Dockerfile -t goonhub:local .
```

The GitHub Actions workflow automatically builds and pushes images to GHCR on pushes to main and version tags.

## Files

```
docker/
├── Dockerfile            # Multi-stage build definition
├── docker-compose.yml    # Development (builds locally)
├── docker-compose.prod.yml # Production (uses GHCR images)
├── config.yaml           # Production config template
├── config-dev.yaml       # Development config (Docker)
├── .env.example          # Secrets template
├── README.md
└── postgres/
    ├── postgresql.conf   # PostgreSQL tuning
    └── init.sql          # Initial SQL setup
```
