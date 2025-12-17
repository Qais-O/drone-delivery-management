# Drone Delivery Management System

A production-ready gRPC service for managing drone deliveries with order assignment, drone tracking, and delivery lifecycle management.

## Features

- **Order Management**: Create, track, and manage delivery orders
- **Drone Fleet Management**: Assign orders to drones with intelligent scheduling
- **Real-time Tracking**: Drone location updates and order status tracking
- **Order Handoff**: Automatic order handoff when drones malfunction
- **ETA Calculation**: Dynamic estimated time of arrival based on drone position and speed
- **JWT Authentication**: Secure gRPC API with token-based auth
- **SQLite Database**: Embedded database with automatic migrations

## Quick Start

### Prerequisites

- Go 1.21 or later
- SQLite3 (bundled with the driver)

### Build

```bash
go build -tags grpcserver -o drone-app ./cmd/server
```

### Run

```bash
# Development (uses insecure JWT secret)
./drone-app

# Production (requires JWT_SECRET environment variable)
JWT_SECRET="your-secret-key" \
DB_PATH="/var/lib/drone-app/app.db" \
GRPC_ADDRESS=":50051" \
./drone-app
```

## Configuration

Configuration is managed via environment variables with sensible defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | `dev-secret-change-me` | JWT signing secret (set in production!) |
| `DB_PATH` | `app.db` | SQLite database file path |
| `GRPC_ADDRESS` | `:50051` | gRPC server listen address |

### Example `.env` file

```bash
JWT_SECRET=your-production-secret-key
DB_PATH=/var/lib/drone-app/app.db
GRPC_ADDRESS=:50051
```

Load it before running:

```bash
export $(cat .env | xargs)
./drone-app
```

## Project Structure

```
droneDeliveryManagement/
├── api/                          # Protocol Buffer definitions
│   ├── admin/v1/                 # Admin service API
│   ├── drone/v1/                 # Drone service API
│   └── user/v1/                  # User order service API
├── cmd/
│   └── server/main.go            # Application entry point
├── internal/
│   ├── auth/                     # JWT authentication & interceptors
│   ├── config/                   # Configuration management
│   ├── db/                       # Database & migrations
│   ├── geo/                      # Geolocation utilities
│   └── grpc/                     # gRPC service implementations
├── models/                       # Domain models
├── repository/                   # Data access layer
├── go.mod & go.sum              # Go module files
└── README.md                     # This file
```

## Architecture

### Layered Architecture

```
gRPC Handlers (internal/grpc)
    ↓
Repositories (repository/)
    ↓
Database (internal/db)

Cross-cutting: Auth (internal/auth), Config (internal/config)
```

### Key Components

1. **Models** (`models/`): Domain entities (Order, Drone, User)
2. **Repositories** (`repository/`): Data access abstraction with query builders
3. **gRPC Services** (`internal/grpc/`): RPC handlers and business logic
4. **Authentication** (`internal/auth/`): JWT validation and authorization
5. **Database** (`internal/db/`): SQLite connection and migrations

## Development

### Build

```bash
go build -tags grpcserver -o drone-app ./cmd/server
```

### Test

```bash
go test ./...
```

### Run Tests with Coverage

```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Database Migrations

Migrations are automatically applied on startup. To add a new migration:

1. Create files: `internal/db/migrations/NNNN_name.up.sql` and `.down.sql`
2. Follow the versioning pattern (0001, 0002, etc.)
3. Restart the application

### Code Style

- Follow Go conventions
- Run `gofmt` before committing
- Use `golint` for linting

## API Reference

### Drone Service

#### ReserveOrder
Assigns the next available order to a drone.

```
rpc ReserveOrder(ReserveOrderRequest) returns (ReserveOrderResponse)
```

#### GrabOrder
Transitions an assigned order from `placed` to `en route` when drone reaches pickup location.

```
rpc GrabOrder(GrabOrderRequest) returns (GrabOrderResponse)
```

#### CompleteOrder
Marks an order as `delivered` or `failed` when drone reaches destination.

```
rpc CompleteOrder(CompleteOrderRequest) returns (CompleteOrderResponse)
```

#### Heartbeat
Updates drone location and speed.

```
rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse)
```

#### GetAssignedOrder
Retrieves details of the currently assigned order with ETA.

```
rpc GetAssignedOrder(GetAssignedOrderRequest) returns (GetAssignedOrderResponse)
```

#### MarkBroken
Marks a drone as broken and hands off any en route order.

```
rpc MarkBroken(MarkBrokenRequest) returns (MarkBrokenResponse)
```

### User Service

#### SetOrder
Creates or updates a delivery order.

```
rpc SetOrder(SetOrderRequest) returns (SetOrderResponse)
```

#### GetOrders
Retrieves user's orders with pagination.

```
rpc GetOrders(GetOrdersRequest) returns (GetOrdersResponse)
```

### Admin Service

See `api/admin/v1/admin_service.proto` for admin operations.

## Security

### Authentication

All gRPC endpoints require a Bearer JWT token in metadata:

```
Authorization: Bearer <token>
```

**Token Claims Required:**
- `name`: User/drone identifier
- `kind`: "admin", "enduser", or "drone"

### Production Checklist

- [ ] Set `JWT_SECRET` to a strong random value
- [ ] Use TLS for gRPC (configure in `internal/grpc/server.go`)
- [ ] Use a persistent database location (e.g., `/var/lib/drone-app/app.db`)
- [ ] Enable database backups
- [ ] Use environment-based configuration
- [ ] Monitor application logs
- [ ] Rate limit gRPC endpoints if exposed

## Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -tags grpcserver -o drone-app ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/drone-app .
ENV JWT_SECRET=must-set-in-production
EXPOSE 50051
CMD ["./drone-app"]
```

Build and run:

```bash
docker build -t drone-app:latest .
docker run -e JWT_SECRET="your-secret" -p 50051:50051 drone-app:latest
```

### Kubernetes

Example deployment manifest:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: drone-app
spec:
  selector:
    app: drone-app
  ports:
    - port: 50051
      targetPort: 50051
  type: ClusterIP

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: drone-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: drone-app
  template:
    metadata:
      labels:
        app: drone-app
    spec:
      containers:
      - name: drone-app
        image: drone-app:latest
        ports:
        - containerPort: 50051
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: drone-app-secrets
              key: jwt-secret
        - name: DB_PATH
          value: /var/lib/drone-app/app.db
        volumeMounts:
        - name: db
          mountPath: /var/lib/drone-app
      volumes:
      - name: db
        persistentVolumeClaim:
          claimName: drone-app-db
```

## Troubleshooting

### Database Lock Error
If you see "database is locked", check:
- Only one instance of the application is running
- The database file has proper permissions
- No other processes are holding the database lock

### Authentication Errors
- Ensure the JWT token is valid and not expired
- Check the `JWT_SECRET` matches between token generation and validation
- Verify token claims contain `name` and `kind` fields

### Connection Refused
- Check `GRPC_ADDRESS` is correct
- Ensure the port is not blocked by firewall
- Verify the application started successfully

## Quick Commands Reference

### All Makefile Targets
```bash
make help              # Show all available commands
make build            # Compile binary
make run              # Build and run
make test             # Run tests
make test-coverage    # Generate coverage report
make coverage-html    # Open HTML coverage
make fmt              # Format code
make lint             # Lint code
make vet              # Vet code
make proto            # Generate protobuf
make clean            # Clean artifacts
make deps             # Download dependencies
make mod-tidy         # Tidy modules
make docker-build     # Build Docker image
make docker-run       # Run Docker container
make check            # Run all checks (fmt + vet + lint + test)
make install-tools    # Install dev tools
```

### Common Development Tasks

**Add New Proto Message**
1. Edit `.proto` file in `api/*/`
2. Run `make proto`
3. Update corresponding service handler

**Add New Database Migration**
1. Create `internal/db/migrations/NNNN_name.up.sql`
2. Create `internal/db/migrations/NNNN_name.down.sql`
3. Restart application (migrations auto-apply)

**Add New Repository Method**
1. Add interface in `repository/interfaces.go`
2. Implement in `repository/*_repository.go`
3. Add tests in `repository/*_repository_test.go`

**Add New Configuration**
1. Add to `internal/config/config.go` struct
2. Add environment variable in `Load()` function
3. Document in `README.md` and `.env.example`

### Code Standards

**File Naming**
- `*_repository.go` - Repository implementations
- `*_server.go` - gRPC service implementations
- `*_test.go` - Test files

**Naming Conventions**
- Packages: lowercase, single word
- Types: PascalCase
- Functions: PascalCase (public), camelCase (private)
- Constants: UPPER_SNAKE_CASE
- Variables: camelCase
- Interfaces: Name ending with `I` (e.g., `UserRepositoryI`)

**Package Organization**
- `internal/` - Private packages
- `api/` - Public interfaces (gRPC)
- `models/` - Domain entities
- `repository/` - Data access layer

### Performance Optimization

**Build Size**
```bash
# Optimized build
go build -ldflags="-s -w" -o drone-app ./cmd/server

# Docker image is already optimized (~30MB)
```

**Database** (already configured in `internal/db/db.go`)
- WAL mode enabled for concurrency
- Busy timeout set to 5 seconds
- Foreign keys enforced

**Deployment**
- Use persistent volumes for database: `docker run -v drone-db:/var/lib/drone-app drone-app:latest`
- Use multiple replicas for high availability (Kubernetes)

## Contributing

1. Follow Go conventions
2. Add tests for new features
3. Update documentation
4. Run `go test ./...` before submitting

## License

MIT - All rights reserved.

## Support

For issues, questions, or feature requests, please contact me at omari.qais@gmail.com.

