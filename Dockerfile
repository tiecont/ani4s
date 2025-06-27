# =============================================================================
# SECURE MULTI-STAGE DOCKERFILE
# =============================================================================

# -----------------------------------------------------------------------------
# Build Stage
# -----------------------------------------------------------------------------
FROM golang:1.24.2-alpine AS builder

# Install only essential build dependencies
RUN apk update && apk add --no-cache \
    ca-certificates \
    git \
    tzdata \
    && rm -rf /var/cache/apk/*

# Set working directory
WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with security flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o app ./main.go

# -----------------------------------------------------------------------------
# Development Stage (for development with hot reload)
# -----------------------------------------------------------------------------
FROM golang:1.24.2 AS development
RUN apt update && apt install -y postgresql-client

# Install Air for live reload
RUN go install github.com/air-verse/air@latest && \
    mv $(go env GOPATH)/bin/air /usr/local/bin/

# Set the working directory
WORKDIR /app

# Copy Air config
COPY .air.toml ./

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod tidy && go mod download

# Copy the rest of the application
COPY . .

# Ensure Air config exists
RUN air init || true

# Copy the entrypoint script and set permissions
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh && ls -l /app/entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["/bin/sh", "-c", "/app/entrypoint.sh"]

# -----------------------------------------------------------------------------
# Production Stage (distroless for maximum security)
# -----------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS production

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary from builder stage
COPY --from=builder /build/app /app

# Use non-root user (distroless nonroot user)
USER 65532:65532

# Expose port
EXPOSE 2500

# Health check for production
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/app", "--health-check"] || exit 1

# Run the application
ENTRYPOINT ["/app"]


# -----------------------------------------------------------------------------
# Scratch-based Production (even more minimal)
# -----------------------------------------------------------------------------
FROM scratch AS production-minimal

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy static binary
COPY --from=builder /build/app /app

# Expose port
EXPOSE 2500

# Run as non-root (this requires your app to handle user switching)
USER 1001:1001

# Run the application
ENTRYPOINT ["/app"]