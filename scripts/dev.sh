#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

for command in docker go air bun ffmpeg ffprobe setsid; do
	if ! command -v "$command" >/dev/null 2>&1; then
		printf 'Missing required command: %s. Install it and ensure it is on PATH.\n' "$command" >&2
		exit 1
	fi
done

docker compose version >/dev/null
export GOONHUB_CONFIG="${GOONHUB_CONFIG:-docker/config-dev.yaml}"

if [[ ! -f "$GOONHUB_CONFIG" ]]; then
	printf 'Configuration file not found: %s\n' "$GOONHUB_CONFIG" >&2
	exit 1
fi

api_pid=""
web_pid=""

cleanup() {
	trap - EXIT INT TERM
	for pid in "$api_pid" "$web_pid"; do
		if [[ -n "$pid" ]]; then
			kill -TERM -- "-$pid" 2>/dev/null || true
		fi
	done
	for pid in "$api_pid" "$web_pid"; do
		if [[ -n "$pid" ]]; then
			wait "$pid" 2>/dev/null || true
		fi
	done
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

compose=(docker compose --project-name "${GOONHUB_DEV_PROJECT:-goonhub-dev}" -f docker/compose.dev.yml)
"${compose[@]}" up -d --wait postgres meilisearch

# Discover published ports instead of competing with other local projects.
database_address="$("${compose[@]}" port postgres 5432)"
meilisearch_address="$("${compose[@]}" port meilisearch 7700)"
export GOONHUB_DATABASE_HOST=127.0.0.1
export GOONHUB_DATABASE_PORT="${database_address##*:}"
export GOONHUB_MEILISEARCH_HOST="http://${meilisearch_address}"
printf 'Development PostgreSQL: %s; Meilisearch: %s\n' "$database_address" "$GOONHUB_MEILISEARCH_HOST"

if [[ ! -x web/node_modules/.bin/nuxt ]]; then
	bun install --cwd web --no-save
fi

printf 'Starting development servers: frontend http://localhost:3000, backend http://localhost:8080\n'
printf 'Press Ctrl+C to stop both servers. Docker dependencies will keep running.\n'

# Separate process groups let shutdown reach the watchers and their children.
setsid air &
api_pid=$!
setsid bun run --cwd web dev &
web_pid=$!

# Stop the sibling server if either watcher exits, preserving the exit status.
set +e
wait -n "$api_pid" "$web_pid"
status=$?
set -e
exit "$status"
