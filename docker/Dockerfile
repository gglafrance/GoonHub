# syntax=docker/dockerfile:1

# ============================================================================
# Stage 1: Build frontend
# ============================================================================
# Pin to host platform — output is static HTML/JS/CSS, architecture-independent
FROM --platform=$BUILDPLATFORM oven/bun:1.3.8-alpine AS frontend-builder

WORKDIR /app/web

# Copy package files first for better caching
COPY web/package.json web/bun.lock* ./

# Install dependencies with BuildKit cache mount for bun
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

# Copy frontend source
COPY web/ ./

# Generate static SPA (ssr: false requires generate, not build)
RUN bun run generate

# ============================================================================
# Stage 2: Build Go backend
# ============================================================================
# Pin to host platform — Go cross-compiles natively without QEMU
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend-builder

ARG TARGETOS
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Ensure Go modules are enabled
ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org,direct

WORKDIR /src

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/dist ./web/dist

# Cross-compile the binary with BuildKit cache mounts
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=gobuild-${TARGETARCH} \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /src/goonhub ./cmd/server

# ============================================================================
# Stage 3: Production runtime
# ============================================================================
FROM alpine:3.23.2

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    ffmpeg \
    chromaprint

# Create non-root user
RUN addgroup -g 1000 goonhub && \
    adduser -u 1000 -G goonhub -h /app -D goonhub

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /src/goonhub /app/goonhub

# Create data directories
RUN mkdir -p /app/data/frames /app/data/thumbnails /app/data/sprites /app/data/vtt /app/data/videos && \
    chown -R goonhub:goonhub /app

# Switch to non-root user
USER goonhub

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/goonhub"]
