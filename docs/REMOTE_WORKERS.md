# Remote workers

GoonHub can hand the heavy phases of scene processing to separate worker processes
(`goonhub-worker`, Windows / macOS / Linux, hardware acceleration when available). This
document is the design and the wire protocol: the server and the worker implement it.

- Go types of the protocol: `pkg/workerproto` (no server import, builds everywhere).
- The computation itself: `pkg/compute`, the same code the server uses for local
  processing (identical commands, names and quality).
- The server commit step: `internal/jobs` (`CommitSprites`, `CommitAnimatedThumbnails`,
  `FingerprintCommitter`), also used by local jobs.

Protocol version: `workerproto.APIVersion = 1`.

## 1. Scope and invariants

Remote phases: `sprites` (sprite sheets + VTT), `animated_thumbnails` (marker clips +
scene preview video), `fingerprint` (chromaprint audio + visual dHash). `metadata` and
`thumbnail` always run on the server.

Two independent choices per worker:

| | |
|---|---|
| **Connection mode** | **pull**: the worker calls the server to claim jobs (machines behind NAT). **push**: the worker runs an HTTP endpoint, the server assigns jobs to it (always-on servers). |
| **Input mode** | **direct**: the worker opens the source through a read-only mount of the server storage (path mapping). **indirect**: the worker reads the source over HTTP from a signed URL with Range support (streamed by ffmpeg for seeking phases, downloaded to a local cache for whole-file phases). |

Every worker request after `register` is the same in both connection modes (heartbeat,
upload, complete...): push only changes who starts the assignment.

Invariants (non-negotiable, resilience first):

1. **Nothing is lost when a worker fails.** Every leased job either completes, fails
   through the normal retry policy, or goes back to `pending`.
2. **A worker can be stopped at any time.** A graceful stop (Ctrl+C, service stop,
   SIGTERM) releases its jobs at once; a kill lets the lease expire.
3. **Fencing.** Every claim gets a new lease generation. Heartbeat, upload, complete, fail
   and release carry it; any other generation than the job's current one is refused
   (`409 lease_lost`). A zombie worker can never overwrite or double-commit.
4. **Staging + atomic commit.** Uploaded artifacts are invisible until `complete` has
   verified every sha256; then files are renamed into place and the DB / ClickHouse writes
   run. The same complete twice is a no-op.
5. **Artifacts always come back by upload**, even in direct mode. The worker never writes
   to the server database, ClickHouse, the generated folders or the media folders.
6. **Media are read-only.** The source is only ever opened for reading (direct mounts
   should be read-only mounts too); the media endpoint serves GET/HEAD only.

## 2. Roles

**Server**

- owns the queue (`job_history`), the leases and the generations;
- prepares each assignment (phase parameters from its config and DB, see section 9);
- serves the source (signed media URL);
- receives artifacts into a staging area, verifies them, commits;
- runs the post-completion steps (job history, events, triggered phases) exactly as for a
  local job.

**Worker**

- registers, claims (pull) or accepts (push) assignments up to its slots;
- computes with `pkg/compute` into a job directory of its outbox;
- uploads artifacts with resume, then calls complete;
- heartbeats every lease; releases on graceful stop; reloads its outbox after a crash.

## 3. Server configuration (`workers.*`)

All timings are server config; the worker receives them in the register response and
never hard-codes them.

| Key | Default | Meaning |
|---|---|---|
| `workers.enabled` | `false` | Enables the worker API. |
| `workers.heartbeat_interval` | `10s` | How often a worker heartbeats (sent to the worker). |
| `workers.lease_duration` | `60s` | A lease not renewed for this long expires. Must be >= 3 x heartbeat_interval. |
| `workers.expiry_sweep_interval` | `5s` | How often the server expires leases. |
| `workers.claim_max_wait` | `25s` | Longest claim long-poll. |
| `workers.media_url_ttl` | `6h` | Validity of a signed media URL. |
| `workers.media_secret` | generated | HMAC secret of media URLs. Empty: generated at the first start and stored in `worker_settings.media_secret` (survives restarts). |
| `workers.max_artifact_bytes` | `2GiB` | Largest single artifact. |
| `workers.upload_chunk_bytes` | `16MiB` | Recommended PUT body size (sent to the worker). |
| `workers.max_complete_body_bytes` | `64MiB` | Largest complete body (fingerprints are inline). |
| `workers.staging_retention` | `24h` | Staging dirs of jobs no longer leased are deleted after this. |
| `workers.max_assignment_age` | `24h` | A lease older than this is no longer renewed (heartbeat answers `lease_lost`). |
| `workers.startup_lease_grace` | = lease_duration | On server start, every remote lease is extended by this, so workers can reconnect. |
| `workers.push_poll_interval` | `5s` | Push dispatcher loop. |
| `workers.push_timeout` | `10s` | Timeout of a push call to a worker. |
| `workers.offline_after` | 3 x heartbeat_interval | No heartbeat for this long: the worker shows offline (its leases expire on their own). |
| `workers.local_phases` | all three | Default routing (section 15): a listed phase is `both` (local pools and workers), a phase not listed is `remote` (workers only). The admin routing overrides it per phase. |
| `workers.allowed_cidrs` | `100.64.0.0/10, fd7a:115c:a1e0::/48, 127.0.0.0/8, ::1/128` | Source addresses accepted on the worker API (Tailscale + loopback), `403 address_not_allowed` otherwise. The client IP follows `server.trusted_proxies`. |
| `workers.public_url` | `""` | Base URL of the server for the workers (signed media URLs). `""` = the scheme and `Host` the worker used on its last `register` (stored per worker). Set it when workers reach the server through a proxy. |
| `processing.staging_dir` | `""` | Staging root (`""` = `staging` next to `processing.sprite_dir`). Remote uploads go under `<staging>/remote/<job_id>/g<generation>/`, local jobs under `<staging>/local/<job_id>/`. |

Every key can be set with an environment variable (`GOONHUB_WORKERS_ENABLED=true`,
`GOONHUB_WORKERS_LOCAL_PHASES=sprites,fingerprint`...). With `workers.enabled: false` the
worker API answers `404 workers_disabled` (the media URL too) and every phase runs locally,
whatever the admin routing: the server behaves exactly as without remote workers. The
lease expiry sweep and the startup grace still run, so leases left from a time the API was
enabled are never lost.

## 4. Transport and authentication

- Machines talk over **Tailscale**. The server API is plain HTTP on the tailnet address
  (MagicDNS name); `workers.allowed_cidrs` rejects anything else with `403`.
- **Per-worker tokens.** An admin creates a worker in the UI / admin API; the server
  returns a token once: `ghw_<worker_id>_<32 random bytes base64url>`. The server stores
  only its sha256. Every worker -> server call sends `Authorization: Bearer <token>`.
  Unknown / revoked token: `401 unauthorized`. Disabled worker: `403 worker_disabled`.
- **Push token.** In push mode the worker generates a random secret at start and sends it
  in `register` (`push_token`); the server sends `Authorization: Bearer <push_token>` on
  every call to the worker, which refuses anything else (`401`). The server keeps it with
  the worker row (it only allows assigning jobs to that worker).
- Every request carries `X-Goonhub-Worker-Api: 1`. An unsupported version on register
  (`api_version`) or in the header of any call: `426 unsupported_api_version`.
- The signed media URL needs no bearer (ffmpeg and fpcalc open it as is): its HMAC
  signature authorizes one job and one generation until an expiry.
- Errors use the GoonHub error body: `{"error": "message", "code": "lease_lost"}`
  (`workerproto.ErrorResponse`, codes in `workerproto.Code*`). An unexpected server error is
  `500` with code `internal_error` (retry with backoff).
- Body limits: `complete` up to `workers.max_complete_body_bytes`, every other JSON body
  4 MiB (`413 invalid_request`). Artifact PUTs have no server read timeout (a slow chunk is
  not cut), the worker API responses are never gzip-compressed (Range on the media URL).
- Times are RFC 3339 UTC; durations are integer seconds in `*_sec` fields.

Admin API (server side, for the UI; not used by the worker; admin role):

| Method + path | Body | Answer |
|---|---|---|
| `GET /api/v1/admin/workers` | - | `{enabled, workers[WorkerView], routing, settings{heartbeat_interval_sec, lease_duration_sec, offline_after_sec, public_url}}` |
| `POST /api/v1/admin/workers` | `{name, connection_mode?, push_url?, allowed_phases?, max_concurrency?}` | `201 {worker, token}` (the token is shown once) |
| `PATCH /api/v1/admin/workers/:id` | any of `{name, enabled, draining, connection_mode, push_url, allowed_phases, max_concurrency, revoke_token}` | `200 WorkerView` |
| `DELETE /api/v1/admin/workers/:id` | - | `200 {"deleted": true}` (job history kept, `worker_id` set NULL) |
| `POST /api/v1/admin/workers/:id/token` | - | `200 {worker, token}`: new token, the old one stops at once, the session is dropped |
| `GET /api/v1/admin/worker-routing` | - | `RoutingView {enabled, effective, defaults, overrides}` |
| `PUT /api/v1/admin/worker-routing` | `{"routing": {"sprites": "remote", "fingerprint": ""}}` | `RoutingView` (`""` removes the override of a phase) |

`WorkerView` is the worker row (no token hash, no push token) plus `status` (`online`: seen
within `offline_after`; `offline`; `disabled`; `revoked`; `never_connected`), `has_token`,
`capabilities`, `path_mappings` and `jobs` (its current leases: job, phase, scene, progress,
stage, input mode, attempt, generation, lease expiry). Counters: `jobs_completed`,
`jobs_failed`, `leases_expired`, `leases_released`, `last_error`.

Disabling a worker, revoking or rotating its token and deleting it release its leases at
once (back to pending, no attempt counted). `allowed_phases` (default all three) is
intersected with the phases the worker advertises; `max_concurrency` (0 = the worker's
config) is the number of jobs the worker runs at once: it is sent in the register and
heartbeat responses (`max_concurrency`) and replaces the worker's own limits, in total and
per enabled phase, within one heartbeat interval (the server then ignores the per-phase
slots the worker registered with); `draining` makes its claims empty and its heartbeats
answer `drain: true`.

**Creating a worker token for testing**: Settings > Workers > Add worker (the token is
shown once), or with an admin session cookie:
`curl -b cookies.txt -H 'Content-Type: application/json' -d '{"name":"test-pc"}' http://<server>:8080/api/v1/admin/workers`
(the `token` field of the answer). Then enable the API on the server
(`GOONHUB_WORKERS_ENABLED=true`) and give the token to the worker.

## 5. Server data model (migration 088, `docs/DATABASE.md`)

Lease fields on `job_history` (one row per job, as today; the attempt of a job is its
`retry_count`):

| Column | Type | |
|---|---|---|
| `worker_id` | `bigint NULL` FK `workers` (SET NULL) | worker holding the lease; NULL = local or unleased; kept on completed / failed / cancelled rows for history |
| `lease_generation` | `bigint NOT NULL DEFAULT 0` | fencing token, bumped on every claim, expiry, release, cancel |
| `lease_expires_at` | `timestamptz NULL` | |
| `last_heartbeat_at` | `timestamptz NULL` | set at claim, then by every heartbeat |
| `committed_generation` | `bigint NULL` | generation whose complete was committed (idempotency) |
| `commit_started_generation` | `bigint NULL` | generation whose commit started (fingerprint at most once) |
| `input_mode` | `varchar(10) NULL` | informational, from the heartbeats |
| `worker_stage` | `varchar(20) NULL` | from the heartbeats |
| `lease_params` | `jsonb NULL` | the `PhaseParams` prepared at claim (the complete is validated against them) |

`workers`: id, name, token_hash (sha256 hex, NULL = revoked, unique), enabled, draining,
connection_mode, push_url, push_token, session_id, server_url, capabilities (jsonb),
path_mappings (jsonb), allowed_phases (text[]), max_concurrency, version, hostname, os,
arch, free_disk_bytes, worker_draining, last_seen_at, registered_at, the counters,
last_error(_at), created_at, updated_at. The online/offline status is computed from
last_seen_at.

`worker_settings` (singleton): media_secret, phase_routing (jsonb overrides).

Uploads are tracked on disk next to the staged files (section 10), not in a table: they
survive a server restart with the staging directory.

The existing recovery ignores remote leases: `MarkOrphanedRunningAsFailed`,
`MarkRunningAsInterrupted` (server shutdown), `ResetJobsToPending` and `CancelClaimedJobs`
only touch rows with `worker_id IS NULL`; remote rows are handled
by lease expiry. Read `docs/DATABASE.md` before the migration.

## 6. Leases, generations, fencing

State of a job row seen by the protocol:

```
pending --claim (gen+1, worker=W, expires=now+lease)--> running(W, gen)
running(W, gen) --heartbeat(gen) ok--> running (expires=now+lease)
running(W, gen) --complete(gen) verified+committed--> completed (committed_generation=gen)
running(W, gen) --fail(gen)--> failed (retry policy: retry_count, retry scheduler, DLQ)
running(W, gen) --release(gen) graceful--> pending (gen+1, worker=NULL, retry_count unchanged)
running(W, gen) --lease expired (sweep)--> pending (gen+1, worker=NULL, retry_count+1)
running(W, gen) --cancel by user--> cancelled (gen+1)
```

Rules:

- **Claim** is one statement: pick pending rows `FOR UPDATE SKIP LOCKED` (same ordering
  as the local feeder: priority, new scenes first, phase windows of the job schedule), set
  `status='running'`, `worker_id`, `lease_generation = lease_generation + 1`,
  `lease_expires_at = now() + lease_duration`, `started_at = now()`. The local feeder and
  the workers compete on the same rows; the status transition guarantees one owner.
- **Every guarded call** is a compare-and-set:
  `UPDATE job_history SET ... WHERE job_id = $1 AND lease_generation = $2 AND worker_id = $3 AND status = 'running'`.
  0 rows: the lease is lost (`409 lease_lost`), or the job was cancelled
  (`409 job_cancelled`), or already committed by this generation (complete only:
  `200 already_committed`).
- **Expiry** (every `expiry_sweep_interval`): `status='pending', worker_id=NULL,
  lease_generation+1, retry_count+1, lease_expires_at=NULL` for running remote rows whose
  `lease_expires_at < now()`. When `retry_count` would exceed `max_retries`, the job goes
  through the normal failure path instead (`failed`, error "lease expired on worker <name>",
  retry scheduler / DLQ). The generation bump at expiry kills the old lease immediately:
  a late heartbeat or complete of the old generation is refused even before anybody
  reclaims the job.
- The generation is a per-job counter (starts at 0, first claim = 1). A worker never
  compares generations of different jobs.
- A lease older than `max_assignment_age` is not renewed: protects against a worker that
  heartbeats forever without progress.
- Server clocks only: the worker uses `server_time` to correct `lease_expires_at` for its
  own clock offset.
- A release or a register that counts an attempt (`crash_recovery`, leases not resumed)
  follows the same rule as the expiry: when `retry_count + 1` would exceed `max_retries`,
  the job fails through the retry path (DLQ) instead of going back to pending.
- On server start every remote lease expiring before `now + startup_lease_grace` is moved
  to `now + startup_lease_grace`; sessions are kept (in the `workers` table), so a worker
  simply goes on with its heartbeats.

## 7. Endpoints (worker -> server)

Base: the server URL the worker is configured with. All bodies are JSON
(`Content-Type: application/json`) except artifact PUT bodies (raw bytes).

### 7.1 Register: `POST /api/v1/workers/register`

Request `RegisterRequest`:

```json
{
  "api_version": 1,
  "name": "gaming-pc",
  "version": "0.1.0",
  "hostname": "DESKTOP-1",
  "os": "windows",
  "arch": "amd64",
  "connection_mode": "pull",
  "push_url": "",
  "push_token": "",
  "capabilities": {
    "phases": ["sprites", "animated_thumbnails", "fingerprint"],
    "slots": {"sprites": 1, "animated_thumbnails": 2, "fingerprint": 2},
    "input_modes": ["direct", "indirect"],
    "accel": "nvidia",
    "hwaccels": ["cuda", "d3d11va", "qsv"],
    "encoders": ["h264_nvenc", "h264_amf", "libx264"],
    "audio_fingerprint": true,
    "ffmpeg_version": "N-124716"
  },
  "path_mappings": [{"server_prefix": "/media/stash", "local_prefix": "Z:\\stash"}],
  "resume": [{"job_id": "5b0c...", "generation": 3}]
}
```

Response `200 RegisterResponse`:

```json
{
  "worker_id": "4",
  "session_id": "c7a1...",
  "api_version": 1,
  "server_time": "2026-11-23T10:00:00Z",
  "heartbeat_interval_sec": 10,
  "lease_duration_sec": 60,
  "claim_max_wait_sec": 25,
  "max_artifact_bytes": 2147483648,
  "upload_chunk_bytes": 16777216,
  "resumed": [{"job_id": "5b0c...", "generation": 3, "status": "ok", "lease_expires_at": "2026-11-23T10:01:00Z"}]
}
```

- A new session replaces the previous one of the worker (calls with the old session get
  `409 unknown_session`). Every lease the worker still holds that is **not** in `resume`
  with a valid generation is released at once (`retry_count + 1`: the worker crashed while
  holding it).
- `path_mappings` must only list the prefixes the worker checked (mount present and
  readable). A worker whose `input_modes` lacks `indirect` only gets jobs whose source is
  under one of its `server_prefix` (directory match: `/media/a` covers `/media/a/x.mp4`,
  not `/media/ab/x.mp4`); without any mapping it gets nothing. Empty `input_modes` means
  both. The worker still chooses per job (section 11).
- Push mode needs `push_url` and `push_token` (`400` otherwise); they replace the stored
  ones on every register.
- `resumed[].status`: `ok` (lease renewed, continue), `lease_lost`, `cancelled`,
  `committed` (already committed: drop the outbox entry).
- `400 invalid_request`, `401`, `403 worker_disabled`, `426 unsupported_api_version`.
- The server stores capabilities and push data; it never assigns a phase outside
  `capabilities.phases`, nor a fingerprint job of a scene with audio to a worker with
  `audio_fingerprint=false`.

### 7.2 Claim (pull): `POST /api/v1/workers/claim`

Request `ClaimRequest`: `{"session_id": "...", "slots": {"sprites": 1, "fingerprint": 0}, "wait_sec": 25}`

Response `200 ClaimResponse`: `{"assignments": [Assignment...]}` (empty array when
nothing came within `wait_sec`).

- At most `slots[phase]` assignments per phase; phases with 0 or absent are not claimed.
  Also capped by `capabilities.slots[phase]` minus the jobs of that phase the worker already
  holds, by the admin `max_concurrency` and `allowed_phases`, by the phase routing (a phase
  routed `local` is never claimed by workers), by the job schedule time windows (a phase
  outside its window is not claimed) and, for fingerprint, by `duplication.enabled`.
- Order: the same as the local feeder (priority, new scenes first, oldest first), with
  **scene affinity** right after the priority: the jobs of scenes the worker already holds
  (or got in this claim) come first, so the phases of one scene tend to go to one worker
  (one download / one mount read).
- Jobs that cannot be assigned are handled at claim and not returned: scene missing or not
  probed yet (`duration 0`): failed as the local feeder does; animated thumbnails with
  nothing to make: completed at once (post-completion included).
- Long-poll: the server answers as soon as at least one job is claimed, or after
  `min(wait_sec, claim_max_wait)` with an empty list.
- A draining or disabled worker gets an empty list.
- `409 unknown_session`: register again.
- If the response is lost (network cut), the claimed leases simply expire and the jobs
  come back with `retry_count + 1`. (The worker sees nothing; this is the only lost-response
  case with a cost, bounded by lease_duration.)

`Assignment`:

```json
{
  "job_id": "5b0c...",
  "phase": "sprites",
  "scene_id": 1234,
  "attempt": 0,
  "lease": {"generation": 3, "expires_at": "2026-11-23T10:01:00Z", "duration_sec": 60},
  "input": {
    "server_path": "/media/stash/a/video.mp4",
    "storage_path_id": 2,
    "relative_path": "a/video.mp4",
    "size": 1073741824,
    "filename": "video.mp4",
    "url": "http://goonhub.tailnet.ts.net:8080/api/v1/workers/media/5b0c...?exp=...&g=3&sig=...",
    "url_expires_at": "2026-11-23T16:00:00Z",
    "video": {"duration_sec": 1800, "width": 1920, "height": 1080, "frame_rate": 29.97,
              "video_codec": "h264", "audio_codec": "aac"}
  },
  "params": {"sprites": {"scene_id": 1234, "tile_width": 320, "tile_height": 180,
             "frame_interval": 10, "frame_quality": 75, "grid_cols": 12, "grid_rows": 12,
             "concurrency": 0}},
  "assigned_at": "2026-11-23T10:00:00Z"
}
```

`params` carries exactly the field of `phase` (section 9).

### 7.3 Heartbeat: `POST /api/v1/workers/heartbeat`

Request `HeartbeatRequest`:

```json
{"session_id": "...", "free_disk_bytes": 52000000000, "draining": false,
 "jobs": [{"job_id": "5b0c...", "generation": 3, "progress": 42, "stage": "computing", "input_mode": "indirect"}]}
```

Response `200 HeartbeatResponse`:

```json
{"server_time": "...", "drain": false,
 "jobs": [{"job_id": "5b0c...", "generation": 3, "status": "ok", "lease_expires_at": "..."}]}
```

- One call renews every lease of the worker. Sent every `heartbeat_interval_sec`
  (also with no job, as a liveness signal; the server updates `last_seen_at`).
- Per job: `ok` (renewed to now + lease_duration; progress stored in `job_history.progress`),
  `lease_lost` (stop, discard), `cancelled` (stop, discard), `committed` (drop the outbox
  entry).
- `drain: true`: the admin asked the worker to stop claiming; running jobs finish.
- **Every heartbeat must list every job the worker holds**, including a push assignment it
  just accepted. A lease the server holds for the worker that is not listed while its last
  heartbeat (or its claim) is older than 2 x `heartbeat_interval` is released at once
  without counting an attempt (a claim or assign answer lost on the way never waits for
  the lease expiry).
- `progress` (0-100), `stage` and `input_mode` are stored on the job (Settings > Workers
  and the active jobs show them).
- `409 unknown_session`: register again (with `resume`).
- A failed heartbeat is retried with backoff (1 s, 2 s, 4 s ... max 10 s, jitter) while the
  local view of the lease is still valid.

### 7.4 Media

**Signed media URL**: `GET|HEAD /api/v1/workers/media/{job_id}?g=<gen>&exp=<unix>&sig=<hex>`

- `sig = hex(HMAC-SHA256(media_secret, "goonhub-worker-media:v1:<job_id>:<gen>:<exp>"))`
  (`workerproto.SignMedia` / `MediaURL` / `VerifyMedia`).
- Valid while the signature matches, `exp` is not passed **and** `g` is the job's current
  lease generation with the job running. Otherwise `403 unauthorized` (bad signature),
  `403 url_expired` (expired), `409 lease_lost` (other generation, job no longer running),
  `409 job_cancelled`, `404 job_not_found` or `410 scene_gone`.
- `url` points to `workers.public_url` when set, else to the scheme and host the worker
  used on its last register.
- Serves the scene's stored file read-only (`http.ServeContent`: `Range`, `If-Range`,
  `206`, `416`, `Accept-Ranges: bytes`, `Content-Length`, `Last-Modified`, `ETag`).
  No write method exists on this path.
- No bearer token: ffmpeg/fpcalc open the URL directly.

**Fresh URL**: `POST /api/v1/workers/jobs/{job_id}/media-url`
`{"generation": 3}` -> `200 {"url": "...", "expires_at": "..."}`; `409 lease_lost`.
The worker asks one when a media request answers `403 url_expired`, or before starting a
download whose URL expires within 10 minutes.

### 7.5 Artifact upload: `PUT /api/v1/workers/jobs/{job_id}/artifacts/{name}`

Headers:

```
Authorization: Bearer <token>
X-Lease-Generation: 3
Upload-Offset: 0               (byte offset of this body in the file)
X-Artifact-Size: 2480123       (total size)
X-Artifact-Sha256: 9f86d0...   (lowercase hex of the whole file)
Content-Type: application/octet-stream
```

Response `200 UploadStatus`: `{"name": "...", "size": 2480123, "received": 2480123, "sha256": "...", "complete": true}`

- `name` must pass `workerproto.ValidateArtifactName` (`[A-Za-z0-9._-]`, no leading dot,
  no `..`, <= 128 chars): `400 invalid_artifact_name` otherwise. Size above
  `max_artifact_bytes`: `413 artifact_too_large`.
- Generation check as every guarded call: `409 lease_lost` / `409 job_cancelled`. The
  upload does not renew the lease (heartbeats do).
- The server writes to `<staging>/remote/<job_id>/g<gen>/.<name>.part` (declared size and
  sha256 in `.<name>.meta`), appending at `Upload-Offset`. `Upload-Offset` must equal the bytes already stored, else
  `409 offset_mismatch` with the current `UploadStatus` in `details.received` (the worker
  resumes from there). Every byte received before a broken connection is kept (flushed),
  so a cut mid-body is resumed, not restarted.
- A PUT may carry the rest of the file in one body or a chunk (<= `upload_chunk_bytes`
  recommended). Declared size / sha256 must be the same on every chunk of an upload, else
  `409 offset_mismatch` (start again from 0 with a new declaration: the server truncates
  when it receives `Upload-Offset: 0`).
- Bytes beyond the declared size are ignored. When `received == size`: the server hashes
  the file; a match renames `.part` to `name`
  (`complete: true`); a mismatch deletes it and answers `422 sha256_mismatch` (the worker
  uploads again from 0, after re-hashing its local file).
- Disk full on the server: `507 server_disk_full`; the worker keeps its outbox and retries
  with backoff while the lease is alive.
- Re-uploading a complete artifact (same size and sha256) answers `200 complete: true`
  without reading the body beyond the declared size (idempotent).

**Status / resume**: `GET /api/v1/workers/jobs/{job_id}/artifacts/{name}?generation=3`
-> `200 UploadStatus` (`received` = bytes stored, 0 when unknown). Used after a restart or
an error to find the offset.

### 7.6 Complete: `POST /api/v1/workers/jobs/{job_id}/complete`

Request `CompleteRequest`:

```json
{
  "generation": 3,
  "phase": "sprites",
  "result": {"sprites": {"sprite_sheets": ["1234_sheet_001.webp"], "vtt": "1234_thumbnails.vtt",
             "sheet_count": 1, "frame_interval": 10}},
  "artifacts": [
    {"name": "1234_sheet_001.webp", "size": 2480123, "sha256": "9f86d0..."},
    {"name": "1234_thumbnails.vtt", "size": 4120, "sha256": "0b1c2d..."}
  ],
  "stats": {"input_mode": "indirect", "accel": "nvidia", "compute_ms": 52000, "upload_ms": 900}
}
```

Server steps, in order:

1. Auth, generation compare-and-set check (without changing anything yet).
   - Job completed with `committed_generation == generation`: `200 {"status": "already_committed"}`.
   - Other generation / not running: `409 lease_lost` or `409 job_cancelled`.
   - Scene deleted: `410 scene_gone` (the job is cancelled, staging deleted).
2. Validate: `phase` equals the job phase, `result.Validate(phase)`,
   `workerproto.ValidateArtifacts(artifacts, result)`,
   the phase result against the assignment (`SpritesResult.Validate(scene_id)`,
   `AnimatedThumbnailsResult.Validate(&params)` with the params the server prepared at
   claim time, kept with the lease; `FingerprintResult.Validate()`): `422 invalid_result`.
3. Verify every declared artifact is complete in `<staging>/remote/<job_id>/g<gen>/` with
   the declared size and sha256 (the hash computed at upload end; no re-hash needed):
   `422 artifact_missing` with `details` = `{name: "missing" | "partial" | "sha256"}`.
   The worker uploads those again, then retries complete.
4. Record `commit_started_generation = generation` and extend the lease by
   `lease_duration` (the expiry sweep never overtakes a commit), commit (section 10), then
   mark `completed`, `committed_generation = generation`, `worker_id` kept for history. A
   commit error answers `503 commit_failed`; the job stays running with its lease, the
   worker retries complete (commit is idempotent). Animated thumbnails are best effort per
   item as locally: a clip that cannot be committed is logged, not a failure.
5. Post-completion exactly as a local job: `ResultHandler.HandleResult(jobs.JobResult{
   JobID, SceneID, Phase, Status: completed, Data: <committed result>})` (job history,
   SSE event, phase tracker, phases triggered after this one).
6. `200 {"status": "committed"}`. Delete the staging directory of the job.

Idempotency: complete is keyed on `(job_id, generation)`. A retried complete after a lost
response gets `already_committed`. The fingerprint commit inserts into ClickHouse (not
idempotent): the server must run it at most once per generation. Mark the commit in
PostgreSQL first (`commit_started_generation`); a complete that finds the commit of its
generation already started (a crash, a `503` or a lost answer in between) first removes the
scene from the ClickHouse index (`MatchingService.RemoveSceneFromIndex`), then commits
again: the index never holds the scene twice. Completes of one job are serialized.

### 7.7 Fail: `POST /api/v1/workers/jobs/{job_id}/fail`

`FailRequest`: `{"generation": 3, "kind": "compute", "error": "ffmpeg failed ...", "retryable": true}`
-> `200 {}`; `409 lease_lost` (ignore: someone else owns it now).

- Server: `job_history` failed with the error (`worker <name>: <error> (<kind>)`), then
  the existing retry path (`RecordJobFailedWithRetry`: retry scheduler, DLQ after
  `max_retries`). `retryable: false` (e.g. `unsupported`) sets `is_retryable=false` and
  sends the job to the dead letter queue at once. Unknown job: `404 job_not_found`.
- `kind`: `input` (source unreadable), `compute` (ffmpeg/fpcalc failed), `timeout`
  (worker-side timeout), `unsupported` (cannot run this assignment), `internal`.
- The worker only calls fail for errors that would fail on the server too. Its own
  problems (disk full, shutdown, missing binary discovered late) are a release.

### 7.8 Release: `POST /api/v1/workers/jobs/{job_id}/release`, `POST /api/v1/workers/release-all`

`ReleaseRequest`: `{"generation": 3, "reason": "shutdown"}`;
`ReleaseAllRequest`: `{"session_id": "...", "reason": "shutdown"}` -> `200 {"released": 2}`.

- The job goes back to `pending` at once, generation + 1, `worker_id = NULL`.
- `reason`: `shutdown`, `disk_full`, `overloaded` do not count as an attempt;
  `crash_recovery` (the worker restarted mid-compute) counts (`retry_count + 1`), so a
  video that crashes workers ends in the DLQ instead of looping.
- Stale generation: `409 lease_lost` (ignore). Release is idempotent.

## 8. Push mode (server -> worker)

The worker listens on `push_url` (tailnet address). Endpoints (`Authorization: Bearer
<push_token>`, `X-Goonhub-Worker-Api: 1`):

| Method + path | Body | Answer |
|---|---|---|
| `GET /v1/status` | - | `200 PushStatus` `{worker_id, api_version, accepting, free_slots{phase:n}, jobs[{job_id, generation, phase, stage, progress}], accel}` |
| `POST /v1/assign` | `PushAssignRequest {assignment}` | `202 {"accepted": true}`; `409 busy` (no free slot for the phase / not accepting); `400` invalid; `401` |
| `POST /v1/cancel` | `PushCancelRequest {job_id, generation}` | `200` (job stopped and discarded, or unknown: idempotent) |

Server dispatcher (every `push_poll_interval`, per online push worker):

1. `GET /v1/status` (also the liveness check; failures count towards offline).
2. For each phase with free slots: claim jobs for that worker exactly as a pull claim
   (same statement, same lease), then `POST /v1/assign` each.
3. Assign failed (network error, timeout, `409 busy`, `5xx`, anything but `202`): release
   the lease at once (`overloaded`, no attempt counted). If the worker did accept but the
   answer was lost, it will call heartbeat/complete with a generation that was just
   released: refused, it discards. No double processing.

Only enabled push workers that registered (a session, a push URL and a token) are polled,
in parallel. A successful status counts as a sign of life (`last_seen_at`); a failure is
recorded as the worker `last_error`. A worker answering `accepting: false` or drained by
the admin gets nothing. `/v1/cancel` carries the generation the worker held.

After `202`, the job runs exactly as in pull mode: the worker heartbeats, uploads and
completes against the server API. A cancel by a user is sent with `/v1/cancel` (best
effort) and with `cancelled` in the next heartbeat (authoritative).

A push worker still calls `register` at start (it knows the server URL) to get its
session and timings and to resume its outbox.

## 9. Phase parameters, results, preparation

Output names are the ones the server serves today (`workerproto.SpriteSheetName`,
`VTTName`, `MarkerClipName`, `ScenePreviewName`).

### sprites

`SpritesParams {scene_id, tile_width, tile_height, frame_interval, frame_quality,
grid_cols, grid_rows, concurrency}`. Server preparation (as the job queue feeder): tile
size = scene thumbnail size or `CalculateTileDimensions(width, height,
max_frame_dimension_sm)`; `frame_interval`, `grid_cols`, `grid_rows` from
`processing.*`; `frame_quality = frame_quality_sprites`; `concurrency =
sprites_concurrency` (the worker may override: 0 = auto). The computation shortens the
interval for short scenes (`EffectiveSpriteInterval`).

`SpritesResult {sprite_sheets[], vtt, sheet_count, frame_interval}`. Files:
`<scene>_sheet_NNN.webp` (1-based, 3 digits) + `<scene>_thumbnails.vtt` (cues point to
`/sprites/<sheet>#xywh=...`).

Input: seeks only (one `-ss` per frame): ffmpeg reads the URL directly with Range.

Commit: `jobs.CommitSprites(sceneRepo, cfg.SpriteDir, cfg.VttDir, sceneID, res, dir)`
returns the `*jobs.SpritesResult` for `HandleResult`.

### animated_thumbnails

`AnimatedThumbnailsParams {scene_id, markers[{marker_id, timestamp_sec, duration_sec,
output}], marker_width, marker_crf, marker_timeout_sec, preview?{output, segments,
segment_duration, width, crf, with_audio}}`. Server preparation:
`MarkerService.PrepareAnimatedThumbnails(sceneID, forceTarget)` (markers lacking a clip,
all markers when forced; the preview when enabled and missing or forced; nil = nothing to
do, complete the job without assigning it). The params prepared at claim are stored with
the lease (the complete is validated against them).

`AnimatedThumbnailsResult {markers[{marker_id, file}], marker_failures[{marker_id,
error}], preview?{file, with_audio}, preview_error}`. Best effort, as today: a failed clip
or preview does not fail the job. Files: `marker_<id>.mp4`, `<scene>_preview.mp4`.

Input: seeks (one input per preview segment): URL direct.

Commit: `MarkerService.CommitAnimatedThumbnails(sceneID, res, dir)` (wraps
`jobs.CommitAnimatedThumbnails`) -> (clips committed, preview committed, joined errors).

### fingerprint

`FingerprintParams {mode: "audio_only" | "dual"}` (duplication config). Audio
(chromaprint, fpcalc) when `input.video.audio_codec != ""`; visual (dHash, 1 frame / 2 s,
9x8 gray) when there is no audio or in dual mode. Dual mode keeps one fingerprint when the
other fails (`audio_error` / `visual_error`).

`FingerprintResult {audio_fingerprint, visual_fingerprint, audio_duration_sec,
audio_error, visual_error}`; both fingerprints are **base64 of the little-endian bytes**
(the PostgreSQL format), inline in complete (no artifact).

Input: reads the whole file: in indirect mode the worker downloads the source to its
cache first (resumable Range download, see 11), then computes on the local copy.

Commit: `poolManager.FingerprintCommitter().Commit(sceneID, res)` (PostgreSQL save, then
FindMatches / IndexFingerprint per type and ProcessMatches; serialized process-wide).

### Computation entry points

`compute.Run(ctx, env, req, phase, params)` dispatches; or `compute.Sprites`,
`compute.AnimatedThumbnails`, `compute.Fingerprint`. Same functions the server jobs call.

## 10. Staging and commit (server)

```
<staging>/remote/<job_id>/g<generation>/.<name>.part  upload in progress
<staging>/remote/<job_id>/g<generation>/.<name>.meta  declared size + sha256 (+ complete)
<staging>/remote/<job_id>/g<generation>/<name>        uploaded, sha256 verified
<staging>/local/<job_id>/                             local job outputs
```

- Staging is on the generated-data file system (default `staging` next to the sprite
  dir), so a commit is a `rename(2)`: atomic per file, no reader ever sees a partial file.
  Across file systems the commit copies to a temporary file in the destination directory,
  fsyncs, then renames (`jobs.MoveArtifact`).
- Order inside a commit: files first, then DB. Sprites: sheets, then VTT (a VTT never
  points to missing sheets), then `UpdateSprites`. A retried commit finds the files
  already moved (missing in staging, present at the destination) and counts them as done.
- A generation's directory is only read by a complete with that generation: a zombie's
  uploads (old generation) are never committed. The janitor (every
  `min(10 min, max(1 min, staging_retention / 4))`) deletes staging directories whose job
  is no longer leased with that generation, once untouched for `staging_retention`. A
  committed, cancelled or scene-deleted job's staging is deleted at once.
- Never anything under the media storage paths: staging and destinations are generated
  folders only.

## 11. Worker side

### Input selection

1. If a `path_mappings` prefix matches `input.server_path`, map it, open read-only, check
   the size equals `input.size` (and the file is a regular file): **direct**.
2. Otherwise **indirect**: seeking phases (sprites, animated_thumbnails) give ffmpeg the
   URL; fingerprint downloads to the cache (`<cache>/<job_id>/<filename>.part`, Range
   resume from the current size, rename when `size` bytes are there), then computes on it.
   The cache entry is deleted when the job leaves the outbox.

### Outbox (crash safety)

Per job `<data>/jobs/<job_id>/`:

- `assignment.json`: the assignment (written before anything else, fsync).
- `state.json`: `stage` (`computing`, `computed`, `uploading`, `completing`), generation,
  the `CompleteRequest` once computed (result + artifact refs with sha256), per-artifact
  upload offsets (informational: the server's GET status is the truth).
- `out/`: compute output dir (artifacts).

Writes are temp file + fsync + rename. An outbox entry is deleted only when the server
answered `committed` / `already_committed`, or `lease_lost` / `cancelled` / `scene_gone`.

### Start-up after a crash

Load every outbox entry. Entries at `computed` or later are sent in `register.resume`;
for each `ok` the worker resumes uploads (GET status, PUT from `received`) and complete.
Entries still at `computing` are not resumable (partial output): the worker sends them in
`resume` too and, if `ok`, releases them with `crash_recovery` (or restarts the computation
if the lease is still valid and it has a slot: implementation choice). Every other lease
is released by the server at register.

### Graceful stop

1. Stop claiming (push: `accepting=false`, `409 busy` on assign); `draining: true` in
   heartbeats.
2. Cancel running computations (context cancel kills ffmpeg/fpcalc), release those jobs
   (`shutdown`), delete their outbox entries.
3. Jobs already computed: try to finish upload + complete within `shutdown_grace`
   (worker config, default 10 s); release whatever did not finish.
4. `release-all` (`shutdown`) as a safety net, then exit. A second Ctrl+C exits at once
   (the leases expire on their own).

### Timeouts

The worker applies the server's semantics: fingerprint timeout scales with duration (pool
timeout or 0.5 s per video second, whichever is larger); each marker clip has
`marker_timeout_sec`. A worker-side timeout is a `fail` with kind `timeout`.

### Retries

Network errors and `5xx` / `507`: retry with exponential backoff (1 s -> 30 s, jitter)
while the lease is believed valid. The local deadline is the last `lease_expires_at`
corrected by the server clock offset. If no heartbeat succeeded for 2 x lease_duration,
the lease is certainly gone: stop computing; keep a computed outbox entry until the server
answers about it (it will say `lease_lost` and the entry is dropped).

## 12. Timings

| | Default | Owner |
|---|---|---|
| Heartbeat interval | 10 s | server config, sent at register |
| Lease duration | 60 s | server config |
| Expiry sweep | 5 s | server |
| Max lease loss on a kill | lease duration + sweep (~65 s) | |
| Claim long-poll | 25 s | server cap, worker asks |
| Media URL TTL | 6 h | server |
| Upload chunk | 16 MiB recommended | server |
| Backoff | 1 s doubling to 30 s, +-20 % jitter | worker |
| Shutdown grace | 10 s | worker config |
| Startup lease grace | lease duration | server |
| Staging retention | 24 h | server |
| Max assignment age | 24 h | server |

## 13. Failure cases

| Case | Worker | Server | Outcome |
|---|---|---|---|
| Worker killed / crash / power loss mid-compute | - | lease expires after lease_duration, sweep: pending, gen+1, retry_count+1 | job reprocessed (here or elsewhere); after max_retries: failed -> DLQ |
| Worker restarted quickly after a crash | register with `resume` | renews valid leases, releases the others (retry_count+1) | computed results resume upload/complete; partial computes are released |
| Graceful stop (Ctrl+C, SIGTERM, service stop) | cancels computes, releases (`shutdown`), release-all | pending at once, gen+1, no attempt counted | no delay, nothing lost |
| Network cut during compute, shorter than the lease | heartbeats retried with backoff | lease still valid | nothing visible |
| Network cut during compute, longer than the lease | keeps computing until 2 x lease without contact, then stops | lease expires, job reassigned | on reconnect: `lease_lost`, outbox dropped |
| Network cut during upload | retries, resumes at `received` (GET status) | keeps the bytes received | upload resumes, no restart from 0 |
| Network cut after uploads, before complete | complete retried with backoff | lease kept alive by heartbeats when they get through | commit when the network is back |
| Complete response lost | retries complete | `already_committed` | single commit |
| sha256 mismatch (corruption) | re-hash local file, upload from 0 | `422 sha256_mismatch`, part deleted | correct file committed |
| Artifact missing at complete (server lost staging, wrong list) | uploads the listed ones, retries | `422 artifact_missing` + details | completes |
| Server restart / downtime < lease | retries everything with backoff | leases persisted in DB; startup grace extends them; staging kept | nothing lost |
| Server downtime > lease | same | grace extends leases at start; then sweep resumes | workers that reconnect within the grace continue; others' jobs are reassigned |
| Server commit fails (DB down) | `503 commit_failed` -> retries complete | job stays running, lease kept | idempotent commit later |
| Server disk full | `507` -> keeps outbox, retries, heartbeats continue | nothing staged | upload when space is back (bounded by max_assignment_age) |
| Worker disk full | stops claiming; running job releases with `disk_full` | pending, no attempt counted | another worker or later |
| Zombie worker (was partitioned, comes back with an old generation) | every call `409 lease_lost` -> discards | compare-and-set refuses; its staging generation is never committed | no overwrite, no double commit |
| Two workers think they own a job | the one with the old generation is refused everywhere | only the current generation can commit | single commit |
| Claim response lost | - | lease expires | job back after ~lease_duration, retry_count+1 |
| Push assign lost / timeout | may start the job | releases the lease at once | worker refused on its first call, discards |
| User cancels the job | heartbeat says `cancelled` (push: /v1/cancel): kills compute, drops outbox | cancelled, gen+1 | stops within one heartbeat |
| Scene deleted while leased | `410 scene_gone` -> drop | job cancelled, staging deleted | - |
| Source unreadable / ffmpeg error | `fail` (input / compute) | retry policy | same as local |
| Worker lacks fpcalc | does not advertise `audio_fingerprint` | assigns only scenes without audio | - |
| Hardware encoder/decoder fails | `pkg/compute` retries the step in software | - | same output names, software quality |
| Worker token revoked / worker disabled | `401` / `403`: stops claiming, keeps outbox until fixed | releases its leases | - |
| Clock skew | uses `server_time` offset | server clock only | - |

## 14. Go API

### pkg/workerproto

Constants: `APIVersion`; `Phase` (`PhaseSprites`, `PhaseAnimatedThumbnails`,
`PhaseFingerprint`, `RemotePhases`, `Phase.Valid()`); `InputMode` (`InputDirect`,
`InputIndirect`); `ConnectionMode` (`ConnPull`, `ConnPush`); headers `Header*`; paths
`Path*` + `JobPath(template, jobID)`, `ArtifactPath(jobID, name)`; error codes `Code*`;
`JobStage` (`StageDownloading`, `StageComputing`, `StageUploading`, `StageCompleting`);
`LeaseStatus` (`LeaseOK`, `LeaseLost`, `LeaseCancelled`, `LeaseCommitted`);
`CompleteStatus`; `FailureKind`; `ReleaseReason` (+ `CountsAsAttempt()`);
`FingerprintModeAudioOnly`, `FingerprintModeDual`; `DefaultMarkerClipTimeoutSec`;
`MaxArtifactNameLen`.

Messages: `RegisterRequest`, `Capabilities`, `PathMapping`, `JobLeaseRef`,
`RegisterResponse`, `ClaimRequest`, `ClaimResponse`, `Assignment`, `Lease`, `InputSpec`,
`VideoInfo`, `HeartbeatRequest`, `JobHeartbeat`, `HeartbeatResponse`, `JobLeaseStatus`,
`ArtifactRef`, `UploadStatus`, `CompleteRequest`, `JobStats`, `CompleteResponse`,
`FailRequest`, `ReleaseRequest`, `ReleaseAllRequest`, `ReleaseResponse`,
`MediaURLRequest`, `MediaURLResponse`, `PushAssignRequest`, `PushAssignResponse`,
`PushCancelRequest`, `PushStatus`, `PushJobStatus`, `ErrorResponse`.

Phases: `PhaseParams`, `PhaseResult` (+ `Validate(phase)`, `PhaseResult.Files()`),
`SpritesParams`/`SpritesResult`, `AnimatedThumbnailsParams`/`MarkerClipSpec`/
`ScenePreviewSpec`/`AnimatedThumbnailsResult`/`MarkerClipResult`/`MarkerFailure`/
`ScenePreviewResult`, `FingerprintParams`/`FingerprintResult`, `AudioFingerprint`,
`VisualFingerprint` (base64 JSON, `Bytes()`, `*FromBytes`).

Helpers:

```go
func ValidateArtifactName(name string) error            // wraps ErrInvalidArtifactName
func ValidateArtifacts(refs []ArtifactRef, result PhaseResult) error
func ValidSHA256(s string) bool
func SHA256File(path string) (sum string, size int64, err error)
func SHA256Reader(r io.Reader) (sum string, size int64, err error)
func ArtifactRefForFile(name, path string) (ArtifactRef, error)
func SpriteSheetName(sceneID uint, index int) string    // index from 1
func VTTName(sceneID uint) string
func MarkerClipName(markerID uint) string
func ScenePreviewName(sceneID uint) string
func EffectiveSpriteInterval(duration, interval, minFrames int) int
func SignMedia(secret []byte, jobID string, generation int64, expires time.Time) string
func MediaURL(baseURL string, secret []byte, jobID string, generation int64, expires time.Time) string
func VerifyMedia(secret []byte, jobID string, query url.Values, now time.Time) (int64, error)
```

### pkg/compute

```go
type Env struct { FFmpegPath, FFprobePath, FpcalcPath string; Accel Accel; TempDir string }
type Request struct { Input string; Video workerproto.VideoInfo; OutDir string; Progress ProgressFunc }
type ProgressFunc func(percent int)

func Run(ctx context.Context, env Env, req Request, phase workerproto.Phase, params workerproto.PhaseParams) (workerproto.PhaseResult, error)
func Sprites(ctx context.Context, env Env, req Request, p workerproto.SpritesParams) (*workerproto.SpritesResult, error)
func AnimatedThumbnails(ctx context.Context, env Env, req Request, p workerproto.AnimatedThumbnailsParams) (*workerproto.AnimatedThumbnailsResult, error) // partial result + ctx error on cancel
func Fingerprint(ctx context.Context, env Env, req Request, p workerproto.FingerprintParams) (*workerproto.FingerprintResult, error)
func MarkerClip(ctx context.Context, env Env, input, outPath string, timestampSec, durationSec, width, crf int, timeout time.Duration) error
func IsURL(input string) bool
func FailedStep(err error) Step                          // from *StepError
```

Hardware acceleration (`Accel{Name, Steps map[Step]StepAccel, NoSoftwareFallback}`; steps
`StepSpriteFrame`, `StepMarkerClip`, `StepScenePreview`, `StepVisualHash`;
`StepAccel{Decode DecodeAccel{HWAccel, OutputFormat, Device, ExtraInputArgs}, Scale
ScaleAccel{Filter, DownloadFormat}, Encoder EncoderAccel{H264, QualityFlag, QualityScale,
QualityOffset, QualityMin, QualityMax, QualityArgs func(crf int) []string, ExtraArgs}}`).
The zero value is software and produces exactly the server's historical ffmpeg commands
(tested against them). An accelerated step that fails is retried in software.

### Server (internal)

```go
// internal/jobs
func StagingRoot(configured, spriteDir string) string
func LocalStagingDir(stagingRoot, jobID string) string
func MoveArtifact(artifactDir, destDir, name string) error
func CommitSprites(repo SpritesCommitRepo, spriteDir, vttDir string, sceneID uint, res *workerproto.SpritesResult, artifactDir string) (*SpritesResult, error)
func CommitAnimatedThumbnails(c AnimatedThumbnailsCommit, sceneID uint, res *workerproto.AnimatedThumbnailsResult, artifactDir string) (committed int, previewCommitted bool, err error)
func NewFingerprintCommitter(repo FingerprintCommitRepo, logger *zap.Logger) *FingerprintCommitter
func (c *FingerprintCommitter) Commit(sceneID uint, res *workerproto.FingerprintResult) error
type AnimatedThumbnailGenerator interface { PrepareAnimatedThumbnails(...); CommitAnimatedThumbnails(...) }

// internal/core
func (s *MarkerService) PrepareAnimatedThumbnails(sceneID uint, forceTarget string) (*jobs.AnimatedThumbnailsPlan, error)
func (s *MarkerService) CommitAnimatedThumbnails(sceneID uint, res *workerproto.AnimatedThumbnailsResult, artifactDir string) (int, bool, error)
func SceneVideoInfo(scene *data.Scene) workerproto.VideoInfo

// internal/core/processing
func (pm *PoolManager) FingerprintCommitter() *jobs.FingerprintCommitter
func (rh *ResultHandler) HandleResult(result jobs.JobResult) // post-completion, Data = *jobs.SpritesResult / *jobs.FingerprintResult for remote jobs
func (rh *ResultHandler) SetPhaseRouter(r PhaseRouter)

// internal/data (migration 088)
type JobLeaseRepository interface { ClaimForWorker; SetLeaseParams; RenewLease; CheckLease; ReleaseLease; ReleaseWorkerLeases; ExpireLeases; MarkCommitStarted; CompleteLease; FailLease; CancelLease; CancelRemoteLeasesByPhases; ExtendRemoteLeases; ListRemoteRunning; GetJobs }
type WorkerRepository interface { Create; GetByID; List; Update; Delete; IncrementCounter; RecordError; GetSettings; EnsureMediaSecret; UpdatePhaseRouting }

// internal/workers (server side of the protocol)
func NewService(d Deps) *Service       // Register, Claim/ClaimFor, Heartbeat, Release, ReleaseAll, Fail, Complete, MediaURL, MediaFile, CancelJob, CancelPhases, admin ops
func (s *Service) Start(d *PushDispatcher) / Stop()   // startup grace, expiry sweep, staging janitor, push loop
func NewRouter(enabled bool, localPhases []string, repo data.WorkerRepository) *Router
func NewStaging(root string, maxBytes int64) *Staging
func NewPushDispatcher(svc *Service, client *http.Client) *PushDispatcher

// internal/api/v1/handler: WorkerHandler (Auth / Gate middlewares, worker and admin endpoints)
```

Extra error codes of the server: `workerproto.CodeWorkersDisabled` (`404`, API off),
`workerproto.CodeAddressNotAllowed` (`403`), `internal_error` (`500`).

Sprites parameters are built like `JobQueueFeeder.submitJobToPool` does (tile size,
config, quality config); `(*jobs.SpritesJob).Params()` shows the mapping.

## 15. Coexistence with local processing (routing)

The server keeps its own pools. Each remotable phase has a routing mode:

| Mode | Local pool | Remote workers |
|---|---|---|
| `local` | claims | never claim |
| `both` | claims | claim (first claim wins on the same pending rows) |
| `remote` | does not claim (as with 0 local workers); its queued jobs go back to pending | claim |

The mode is the admin override (Settings > Workers, `PUT /api/v1/admin/worker-routing`,
stored in `worker_settings.phase_routing`) or else the config: `workers.local_phases`
listed = `both`, not listed = `remote`. With the worker API disabled every phase is
`local`. `metadata` and `thumbnail` are always local.

- A `remote` phase's pending jobs wait for a worker: the startup stuck-pending recovery
  skips them (like phases outside their time window).
- After metadata, the sprites phase is submitted straight to the local pool only when its
  mode is `local` (the historical path); otherwise it is created pending in the queue like
  the other triggered phases, where pools and workers compete.
- A user cancel (`POST /admin/jobs/:id/cancel`) of a job leased to a worker cancels it in
  the database (generation + 1); the worker learns it from its next heartbeat (push:
  `/v1/cancel` at once). A bulk cancel with running jobs cancels the remote ones too.
- Both paths compute with `pkg/compute` and commit with the same functions, so results are
  identical.
