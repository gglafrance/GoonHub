# Operations Runbook

## Deployment

### Single-Binary Deployment

GoonHub deploys as a single binary with the frontend embedded.

```bash
# 1. Build frontend
cd web && bun install && bun run build

# 2. Build Go binary (embeds web/dist)
go build -o goonhub ./cmd/server

# 3. Deploy binary + config
scp goonhub target-host:/opt/goonhub/
scp config-prod.yaml target-host:/opt/goonhub/
```

### Runtime Requirements

- PostgreSQL 18 accessible from the application
- ffmpeg/ffprobe on PATH
- Writable `./data/` directory (thumbnails, sprites, VTT files)
- Config file path set via `GOONHUB_CONFIG` environment variable

### Configuration for Production

Create a `config-prod.yaml` (do NOT use `config-dev.yaml` defaults):

| Setting | Production Recommendation |
|---------|--------------------------|
| `auth.paseto_secret` | Random 32-byte string (NOT the dev default) |
| `auth.admin_password` | Strong password |
| `database.password` | Unique per-environment |
| `database.sslmode` | `require` or `verify-full` |
| `database.max_open_conns` | 25-50 depending on load |
| `log.level` | `info` or `warn` |
| `log.format` | `json` for structured logging |
| `server.read_timeout` | `30s` |
| `server.write_timeout` | `60s` |

All config keys can be overridden via `GOONHUB_` prefixed env vars.

### Database Migrations

Migrations run automatically on application startup via `golang-migrate`. No manual migration step is needed.

---

## Monitoring

### Health Indicators

| Check | Method | Healthy State |
|-------|--------|---------------|
| Application | HTTP GET any API route | 200 response |
| PostgreSQL | `pg_isready -U goonhub -d goonhub` | Exit code 0 |
| Worker Pool | `GET /api/v1/admin/pool-config` | Returns config |
| Queue Status | Included in video processing service | No stuck jobs |

### Key Metrics to Watch

- **Queue depth**: Jobs waiting per phase (metadata, thumbnail, sprites)
- **Worker pool utilization**: Active vs configured workers
- **Database connections**: Monitor `max_open_conns` vs active connections
- **Disk usage**: `./data/` directory grows with processed videos
- **SSE connections**: Buffered channel capacity is 50; slow subscribers drop events

### Logs

- Format controlled by `log.format` (`console` for dev, `json` for production)
- Level controlled by `log.level`
- Uses `zap` structured logging

---

## Common Issues and Fixes

### Application Won't Start

| Symptom | Cause | Fix |
|---------|-------|-----|
| `failed to connect to database` | PostgreSQL not running or wrong credentials | Check DB is running, verify config |
| `PASETO secret must be 32 bytes` | Invalid `auth.paseto_secret` | Ensure exactly 32 characters |
| `migration failed` | Schema conflict or corrupt state | Check `schema_migrations` table in DB |
| `bind: address already in use` | Port already taken | Change `server.port` or kill existing process |

### Video Processing Failures

| Symptom | Cause | Fix |
|---------|-------|-----|
| `exec: "ffmpeg": executable file not found` | ffmpeg not installed | Install ffmpeg, ensure on PATH |
| `exec: "ffprobe": executable file not found` | ffprobe not installed | Install ffmpeg (includes ffprobe) |
| Jobs stuck in queue | Worker pool stopped or too few workers | Check pool config, restart if needed |
| Thumbnails not generated | Output directory not writable | Check permissions on `./data/` |
| Sprite generation slow | High concurrency on limited CPU | Reduce `sprites_concurrency` in config |

### Authentication Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| Admin can't login | Wrong `admin_password` in config | Update config and restart |
| Token expired immediately | `token_duration` too short | Increase in config |
| SSE not receiving events | Wrong token in query param | Verify `?token=` uses valid auth token |

### Database Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| Connection pool exhausted | Too many concurrent requests | Increase `max_open_conns` |
| Slow queries | Missing indexes or large tables | Check `job_history` retention settings |
| Disk full | Unmanaged data growth | Configure `job_history_retention` (default 7d) |

### Frontend Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| API calls return 404 in dev | Proxy not configured | Ensure Nuxt dev server is proxying to `:8080` |
| Build fails | Missing dependencies | Run `bun install` in `web/` |
| Stale types | Generated types outdated | Run `bun run postinstall` (nuxt prepare) |

---

## Administrative Operations

### Manually Trigger Video Processing

```bash
# Trigger a specific phase for a video
curl -X POST http://localhost:8080/api/v1/admin/videos/{id}/process/{phase} \
  -H "Authorization: Bearer <token>"
```

Phases: `metadata`, `thumbnail`, `sprites`

### Adjust Worker Pool at Runtime

```bash
# Get current config
curl http://localhost:8080/api/v1/admin/pool-config \
  -H "Authorization: Bearer <token>"

# Update worker counts
curl -X PUT http://localhost:8080/api/v1/admin/pool-config \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"metadata_workers": 3, "thumbnail_workers": 2, "sprites_workers": 1}'
```

### View Job History

```bash
curl http://localhost:8080/api/v1/admin/jobs \
  -H "Authorization: Bearer <token>"
```

### Manage Trigger Schedules

```bash
# Get trigger config
curl http://localhost:8080/api/v1/admin/trigger-config \
  -H "Authorization: Bearer <token>"

# Update triggers (supports: on_import, after_job, manual, scheduled)
curl -X PUT http://localhost:8080/api/v1/admin/trigger-config \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"triggers": [...]}'
```

---

## Rollback Procedures

### Application Rollback

1. Stop the current binary
2. Deploy the previous binary version
3. Start with same config
4. Migrations are forward-only; database rollback requires manual intervention

### Database Rollback

Database migrations are **forward-only** by design. To rollback:

1. Stop the application
2. Identify the target migration version in `internal/infrastructure/persistence/migrator/`
3. Manually run down migrations:
   ```bash
   # Connect to database
   docker exec -it goonhub-postgres psql -U goonhub -d goonhub

   # Check current version
   SELECT * FROM schema_migrations;

   # Manual rollback requires applying reverse SQL from migration files
   ```
4. Deploy the older binary version
5. Start the application

### Data Directory Backup

The `./data/` directory contains generated assets (thumbnails, sprites, VTT). These can be regenerated from source videos by re-triggering processing, but backing up saves reprocessing:

```bash
tar -czf data-backup-$(date +%Y%m%d).tar.gz ./data/
```

---

## Docker (Development Database)

### Start

```bash
cd docker && docker compose up -d
```

### Stop (Preserve Data)

```bash
cd docker && docker compose down
```

### Reset (Destroy Data)

```bash
cd docker && docker compose down -v && docker compose up -d
```

### Backup Database

```bash
docker exec goonhub-postgres pg_dump -U goonhub goonhub > backup.sql
```

### Restore Database

```bash
docker exec -i goonhub-postgres psql -U goonhub -d goonhub < backup.sql
```
