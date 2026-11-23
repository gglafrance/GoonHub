#!/usr/bin/env bash
# Builds goonhub-worker for every supported target into dist/.
# Usage: ./build.sh [version]     (default: git describe)
# Needs Go 1.24+ and the GoonHub integration worktree at ../GoonHub-workers (go.mod replace).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
VERSION="${1:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"
TARGETS="windows/amd64 darwin/arm64 darwin/amd64 linux/amd64 linux/arm64"

mkdir -p "$ROOT/dist"
rm -f "$ROOT"/dist/goonhub-worker_* "$ROOT/dist/SHA256SUMS"
for t in $TARGETS; do
  os="${t%/*}"; arch="${t#*/}"
  out="goonhub-worker_${os}_${arch}"
  [ "$os" = windows ] && out="${out}.exe"
  echo "building $out ($VERSION)"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$LDFLAGS" -o "dist/$out" ./cmd/goonhub-worker)
done
cp "$ROOT/config.example.yaml" "$ROOT/dist/"
(cd "$ROOT/dist" && if command -v sha256sum >/dev/null; then sha256sum goonhub-worker_* > SHA256SUMS; else shasum -a 256 goonhub-worker_* > SHA256SUMS; fi)
echo "done: $ROOT/dist"
ls -l "$ROOT/dist"
