# syntax=docker/dockerfile:1

# ============================================================================
# Stage 1: Build frontend
# ============================================================================
FROM oven/bun:1.3.6-alpine AS frontend-builder

WORKDIR /app/web

# Copy package files first for better caching
COPY web/package.json web/bun.lock* ./

# Install dependencies
RUN bun install --frozen-lockfile

# Copy frontend source
COPY web/ ./

# Generate static SPA (ssr: false requires generate, not build)
RUN bun run generate

# ============================================================================
# Stage 2: Build Go backend
# ============================================================================
FROM golang:alpine3.23 AS backend-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build the binary (TARGETOS and TARGETARCH are set by Docker buildx)
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s" \
    -o /app/goonhub \
    ./cmd/server

# ============================================================================
# Stage 3: Production runtime
# ============================================================================
FROM alpine:3.23.2

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    ffmpeg

# Create non-root user
RUN addgroup -g 1000 goonhub && \
    adduser -u 1000 -G goonhub -h /app -D goonhub

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/goonhub /app/goonhub

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
