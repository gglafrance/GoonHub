# Docker Infrastructure

## Quick Start

```bash
cd docker
docker compose up -d
```

Wait for the healthcheck to pass:
```bash
docker compose ps
```

The PostgreSQL instance will be available at `localhost:5432`.

## Connection Details

| Field    | Value                |
|----------|----------------------|
| Host     | localhost            |
| Port     | 5432                 |
| User     | goonhub              |
| Password | goonhub_dev_password |
| Database | goonhub              |
| SSL      | disabled             |

Connect via psql:
```bash
docker exec -it goonhub-postgres psql -U goonhub -d goonhub
```

## PostgreSQL Configuration

The `postgres/postgresql.conf` is tuned for a media library workload on SSD storage:

- **50 max connections** - sufficient for dev, increase for production
- **256MB shared_buffers** - PostgreSQL's internal cache
- **768MB effective_cache_size** - hints to query planner about OS cache
- **16MB work_mem** - per-operation sort/hash memory
- **SSD-optimized** - `random_page_cost = 1.1` reflects SSD random read speed
- **WAL tuning** - 1GB max WAL size with 0.9 checkpoint target for write throughput
- **Slow query logging** - queries over 1000ms are logged

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

## Migrations

Migrations are located in `internal/infrastructure/persistence/migrator/migrations/` and are embedded into the Go binary. They run automatically on startup when using the PostgreSQL driver.

### Adding a new migration

1. Create two files in the migrations directory:
   - `NNNNNN_description.up.sql` - applies the change
   - `NNNNNN_description.down.sql` - reverts the change
2. Use sequential numbering (e.g., `000002_add_tags.up.sql`)
3. Rebuild the application (migrations are embedded via `//go:embed`)

### Migration versioning

The `schema_migrations` table tracks which migrations have been applied. Never modify an already-applied migration file; create a new one instead.
