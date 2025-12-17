# Multi-stage build for the Drone Delivery Management application

# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git sqlite-dev gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -tags grpcserver -ldflags="-s -w" -o drone-app ./cmd/server

# Stage 2: Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/drone-app .

# Create directory for database (if using persistent volume)
RUN mkdir -p /var/lib/drone-app && chown -R nobody:nobody /app /var/lib/drone-app

# Use non-root user for security
USER nobody

# Expose gRPC port
EXPOSE 50051

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD [ -f /var/lib/drone-app/app.db ] || exit 1

# Set default environment variables
ENV DB_PATH=/var/lib/drone-app/app.db
ENV GRPC_ADDRESS=:50051
ENV JWT_SECRET=must-set-in-production

# Run the application
ENTRYPOINT ["./drone-app"]

