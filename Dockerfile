FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git build-base zstd

# Set working directory
WORKDIR /app

# Set permissions for go mod cache
RUN mkdir -p /go/pkg/mod && \
    chmod -R 777 /go/pkg/mod && \
    chmod -R 777 /go

# Copy go mod files
COPY go.mod ./

# Download dependencies and clean up
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && \
    go mod tidy

# Copy source code
COPY . .

# Build the binary with proper permissions
RUN CGO_ENABLED=0 GOOS=linux go build -o vault-utils -ldflags="-w -s" .

# Create final minimal image
FROM alpine:3.19

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -u 10001 appuser

# Copy binary from builder
COPY --from=builder /app/vault-utils /usr/local/bin/

# Create directory for unseal keys with correct permissions
RUN mkdir -p /vault/unseal-keys && \
    chown -R appuser:appuser /vault/unseal-keys && \
    chmod 700 /vault/unseal-keys

# Switch to non-root user
USER appuser

# Expose health check port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/vault-utils"] 