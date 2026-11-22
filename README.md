# goonhub-worker

Remote job worker for [GoonHub](../GoonHub-workers): runs the heavy processing phases of a
GoonHub server on another machine (Windows, macOS, Linux; amd64 and arm64):

- **sprites**: sprite sheets + VTT (scrubber and seek previews),
- **animated_thumbnails**: marker clips + scene preview video,
- **fingerprint**: chromaprint audio fingerprint + visual dHash (duplicate detection).

The computation is GoonHub's own `pkg/compute` (same ffmpeg commands, names and quality as
the server). The worker never touches the server's database or folders: results always go
back by upload, and the server commits them atomically after checking every sha256.
The protocol is described in `docs/REMOTE_WORKERS.md` of the GoonHub repository.

Resilience first: the worker can be stopped at any time. A graceful stop (Ctrl+C, service
stop, SIGTERM) cancels the running computations and gives their jobs back to the server at
once; a crash or a kill lets the server's lease expire (about a minute) and the job is
requeued. Computed results wait in an on-disk outbox until the server acknowledged them,
uploads resume where they stopped, and a stale worker can never overwrite a newer result
(lease generations).

## Build

Go 1.24+. The module builds against the GoonHub integration worktree
(`replace goonhub => ../GoonHub-workers` in `go.mod`), so keep both checkouts side by side.

```sh
./build.sh            # or .\build.ps1 on Windows (PowerShell)
```

Produces `dist/goonhub-worker_<os>_<arch>[.exe]` for windows/amd64, darwin/arm64,
darwin/amd64, linux/amd64, linux/arm64 (static, `CGO_ENABLED=0`), `SHA256SUMS` and
`config.example.yaml`. A single native build: `go build ./cmd/goonhub-worker`.

## Install

1. **ffmpeg + ffprobe** (required) and **fpcalc** (optional, audio fingerprints). Put them
   next to the worker binary (or in `bin/` beside it), on `PATH`, or set `tools.*` in the
   config.

   | OS | ffmpeg / ffprobe | fpcalc (chromaprint) |
   |---|---|---|
   | Windows | `winget install Gyan.FFmpeg`, or [BtbN builds](https://github.com/BtbN/FFmpeg-Builds/releases) `ffmpeg-master-latest-win64-gpl.zip` (has cuda/nvenc, qsv, amf, d3d11va) | [chromaprint releases](https://github.com/acoustid/chromaprint/releases): `chromaprint-fpcalc-*-windows-x86_64.zip` |
   | macOS | `brew install ffmpeg` (VideoToolbox), or static builds from [evermeet.cx](https://evermeet.cx/ffmpeg/) | `brew install chromaprint`, or `chromaprint-fpcalc-*-macos-universal.tar.gz` |
   | Linux | `apt install ffmpeg` / `dnf install ffmpeg`, or [johnvansickle static builds](https://johnvansickle.com/ffmpeg/), BtbN linux builds | `apt install libchromaprint-tools` / `dnf install chromaprint-tools`, or `chromaprint-fpcalc-*-linux-x86_64.tar.gz` |

   Without fpcalc the worker tells the server it has no audio fingerprinting: the server
   then only sends it fingerprint jobs of scenes without an audio track.

2. Copy the binary and `config.example.yaml` (as `config.yaml`) into a directory, e.g.
   `C:\goonhub-worker\`, `/opt/goonhub-worker/`, `~/goonhub-worker/`.

3. In GoonHub: enable the worker API (`workers.enabled: true` or
   `GOONHUB_WORKERS_ENABLED=true`), then **Settings > Workers > Add worker**. Copy the token
   (shown once) into `token_file` (recommended) or `token`.

4. Check: `goonhub-worker detect --config config.yaml` lists the binaries found, ffmpeg's
   hardware acceleration methods and encoders, the acceleration in use and the direct
   storage mounts.

5. Run in the foreground: `goonhub-worker run --config config.yaml`, or install the service.

### Service

```sh
goonhub-worker service install --config /opt/goonhub-worker/config.yaml   # then: service start
goonhub-worker service stop|start|restart|status|uninstall
```

- **Windows**: a Windows service (`goonhub-worker`), automatic delayed start, restart on
  failure. Run from an elevated prompt. A service runs as LocalSystem by default: mapped
  drives are not visible to it, use UNC paths (`\\nas\stash`) in `storage` and
  `--username` if the share needs credentials.
- **macOS**: launchd (`/Library/LaunchDaemons`, root) or, with `--user`, a launch agent of
  the current user (`~/Library/LaunchAgents`): use `--user` when the media are on a share
  mounted in your session.
- **Linux**: systemd unit (`After=network-online.target tailscaled.service`,
  `Restart=on-failure`), `--user` for `systemctl --user`.

The service stop is graceful (jobs released). The service manager's own stop timeout must
be longer than `shutdown_grace` + a few seconds (systemd's 90 s default is fine).

## Configuration

See [`config.example.yaml`](config.example.yaml); every key is documented there. The
essentials:

```yaml
server: http://goonhub.your-tailnet.ts.net:8080
token_file: token.txt
mode: pull                # or push
phases: {sprites: 1, animated_thumbnails: 1, fingerprint: 2}
max_concurrency: 2
storage:                  # optional: direct mode
  - storage_path_id: 2
    local_path: 'Z:\stash'
cache: {max_size: 20GiB}
```

### Connection modes

- **pull** (default): the worker registers, then long-polls `claim` for its free slots.
  Works behind NAT; only outgoing connections to the server.
- **push**: the worker listens on `push.listen` and registers `push.url` with a random push
  token (or `push.secret`); the server's dispatcher checks `/v1/status` and posts jobs to
  `/v1/assign`. Only requests carrying the token, from `push.allowed_cidrs` (Tailscale and
  loopback by default), are accepted. For always-on machines the server can reach.

Both modes then run the same path: heartbeats, uploads, complete.

### Input modes (chosen per job)

- **direct**: the worker sees the server's storage itself. Map a GoonHub storage path
  (by `storage_path_id`, or by `server_prefix`) to a local **read-only** mount. The file is
  used only when it exists with the size the server knows; otherwise the job silently falls
  back to indirect. The worker only opens sources for reading; it refuses a `data_dir` or
  `cache.dir` inside a media mount.
- **indirect**: the worker reads the video from a signed URL of the server (HTTP Range).
  Sprites and animated thumbnails let ffmpeg seek in the URL; fingerprints download the
  whole file into the cache first (resumable). A cached copy of a scene is reused by every
  later job of that scene (the server hands the phases of one scene to the same worker
  when it can); the cache keeps the most recently used copies within `cache.max_size`.

### Hardware acceleration

`accel: auto` uses the best profile detected for the machine's ffmpeg (NVENC/NVDEC, AMF,
QSV, VideoToolbox, VAAPI) and falls back to software for any step that fails, so the output
is always produced. This build still runs everything in software: the detection lives in
GoonHub's `pkg/compute/hwaccel` (in progress) and plugs into `internal/accel.Select`.
`goonhub-worker bench` times each computation on a generated clip (or `--input FILE`).

## Tailscale

The worker API of the server accepts only Tailscale (100.64.0.0/10, fd7a:115c:a1e0::/48)
and loopback addresses by default (`workers.allowed_cidrs`).

1. Install Tailscale on the worker machine and log in to the same tailnet as the server.
2. Use the server's MagicDNS name in `server:` (`http://<server-name>.<tailnet>.ts.net:8080`).
   Plain HTTP is fine inside the tailnet (WireGuard encrypts it).
3. Push mode: set `push.url` to the worker's MagicDNS name and `push.listen` to `:9090`
   (or its Tailscale IP to listen on the tailnet only). Allow the port in the OS firewall
   for the Tailscale interface.
4. Check from the worker: `curl -X POST http://<server>:8080/api/v1/workers/register` must
   answer quickly (401 without a token, 403 when the address is not allowed, 404 when the
   worker API is off), not time out.

If the server sits behind a reverse proxy, set `workers.public_url` on the server so the
signed media URLs point to an address the worker can reach.

## Stopping and what happens to jobs

| Event | What the worker does | Result on the server |
|---|---|---|
| Ctrl+C / SIGTERM / service stop | cancels computations, releases them (`shutdown`), lets computed jobs finish their upload for `shutdown_grace`, then releases the rest and calls `release-all` | jobs back to pending at once, no attempt counted |
| Second Ctrl+C | exits at once | leases expire (~1 min), jobs requeued (attempt + 1) |
| Crash, kill, power loss | - | lease expiry requeues; on restart the worker resumes computed results whose lease is still valid and releases interrupted computations (`crash_recovery`) |
| Network cut | retries with backoff (1 s to 30 s), resumes uploads and downloads at the right offset | leases kept while heartbeats get through; after 2 x lease without contact the worker stops computing and the server reassigns |
| Server restart | re-registers with its outbox | valid leases continue, others are dropped by the worker |
| Disk below `min_free_disk` | stops taking jobs; a job that fails on a full disk is released (`disk_full`) | no attempt counted |

## Files

```
<config dir>/data/jobs/<job>/   outbox: assignment.json, state.json, out/ (artifacts), tmp/
<config dir>/data/cache/        download cache (copies of sources, indirect mode)
<config dir>/data/logs/worker.log   JSON log, rotated (log.max_size_mb, log.max_backups)
```

Deleting `data/cache` is always safe. Deleting `data/jobs` while the worker is stopped
loses only unsent results (the server's leases expire and the jobs are redone).

## Troubleshooting

- **`ffmpeg not found`**: install it (table above), or set `tools.ffmpeg` / `tools.ffprobe`.
  `goonhub-worker detect` shows what was found.
- **`401`**: wrong or revoked token (rotate it in Settings > Workers).
  **`403 worker_disabled`**: the admin disabled the worker. **`403 address_not_allowed`**:
  the worker does not come through Tailscale (check `server:` uses the tailnet name).
  **`404 workers_disabled`**: enable `workers.enabled` on the server. The worker keeps its
  outbox and retries every minute in all these cases.
- **Jobs never arrive**: check the phase routing in Settings > Workers (a phase routed
  `local` is never given to workers), the worker's `phases`, its free disk, and that it is
  not draining. Pull mode logs each claim at `log.level: debug`.
- **Direct mode not used**: the log says why (`direct input not usable`): no mapping,
  file missing, or size different from the server's. On a Windows service, mapped drive
  letters do not exist: use UNC paths.
- **Audio fingerprints missing**: install fpcalc; `detect` must show it.
- **Push mode: the server cannot reach the worker**: `push.url` must be reachable from the
  server (Tailscale name and port, firewall), `goonhub-worker detect` shows the config;
  the worker logs every refused push call.
- Logs: console and `data/logs/worker.log` (JSON). `log.level: debug` for details.

## Development

```sh
go vet ./... && go test ./...                 # unit + end-to-end tests (fake server)
```

`internal/fakeserver` implements the worker API in memory (leases, generations, expiry,
signed media, resumable staged uploads, verified complete, push dispatch) with fault
injection; `internal/worker/e2e_test.go` drives a real worker against it: pull and push
jobs, kill mid-compute (lease expiry), quick crash restart, crash after compute (resumed
upload), graceful stop, network cut during upload and download (resume), server restart,
zombie generations, cancel, cache reuse, direct vs indirect. Tests named `*Real` run the
real computation with ffmpeg and skip without it; the audio test skips without fpcalc.
The race detector needs cgo: on Windows run it in Linux, e.g.

```sh
MSYS_NO_PATHCONV=1 docker run --rm -v "$PWD":/src/goonhub-worker -v "$PWD/../GoonHub-workers":/src/GoonHub-workers:ro \
  -w /src/goonhub-worker golang:1.24 sh -c "apt-get update -qq && apt-get install -y -qq ffmpeg libchromaprint-tools >/dev/null && go test -race ./..."
```

`cmd/goonhub-worker/signal_unix_test.go` builds the binary and checks the real stop
signals (process-group SIGINT like a terminal Ctrl+C, and the double signal).
