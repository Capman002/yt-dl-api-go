# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates (required for go mod download)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# CGO_ENABLED=0 produces a static binary
# -ldflags="-s -w" strips debug info for smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/api

# Stage 2: Runtime image
FROM alpine:3.19

# Install runtime dependencies
# Cache bust: 2026-01-12-v2
RUN apk add --no-cache \
    ca-certificates \
    ffmpeg \
    python3 \
    py3-pip \
    && pip3 install --no-cache-dir --break-system-packages yt-dlp \
    && echo "yt-dlp location:" && which yt-dlp \
    && yt-dlp --version \
    && ln -sf $(which yt-dlp) /usr/local/bin/yt-dlp || true

# Create non-root user
RUN addgroup -g 1000 app && \
    adduser -u 1000 -G app -s /bin/sh -D app

# Create directories
RUN mkdir -p /app/data /app/tmp && \
    chown -R app:app /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /api /app/api

# Switch to non-root user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

# Run the binary
CMD ["/app/api"]
