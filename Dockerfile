# ==============================================================================
# Build Stage
# ==============================================================================
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /build/vpsctl \
    ./cmd/vpsctl

# ==============================================================================
# Runtime Stage
# ==============================================================================
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    bash \
    openssh-client \
    curl \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 vpsctl \
    && adduser -u 1000 -G vpsctl -s /bin/bash -D -h /home/vpsctl vpsctl

# Create required directories
RUN mkdir -p /etc/vpsctl/certs \
    /var/lib/vpsctl/backups \
    /var/log/vpsctl \
    /home/vpsctl \
    && chown -R vpsctl:vpsctl /etc/vpsctl \
    /var/lib/vpsctl \
    /var/log/vpsctl \
    /home/vpsctl

# Copy binary from builder (frontend assets are embedded; binary is self-contained)
COPY --from=builder /build/vpsctl /usr/local/bin/vpsctl

# Switch to non-root user
USER vpsctl

# Expose the default port
EXPOSE 8443

# Environment variables
ENV VPSCTL_CONFIG=/etc/vpsctl/config.yaml \
    VPSCTL_DATA_DIR=/var/lib/vpsctl \
    VPSCTL_LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fs http://localhost:8080/api/health || exit 1

# Default command
ENTRYPOINT ["vpsctl"]
CMD []
