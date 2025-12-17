# Quick Reference Guide

## 📋 Build & Run

### Build
```bash
make build                    # Build application
go build -tags grpcserver -o drone-app ./cmd/server
```

### Run
```bash
make run                      # Build and run

# Or with environment variables
JWT_SECRET="prod-secret" \
DB_PATH="/var/lib/app/app.db" \
./drone-app
```

### Test
```bash
make test                     # Run tests
make test-coverage           # Coverage report
make check                   # fmt + vet + lint + test
```

### Docker
```bash
make docker-build            # Build Docker image
docker run -e JWT_SECRET="secret" -p 50051:50051 drone-app:latest
```

---

## 🔧 Configuration

### Environment Variables
```bash
export DB_PATH=app.db                              # SQLite path (default)
export GRPC_ADDRESS=:50051                         # Server address (default)
export JWT_SECRET=your-production-secret-key       # JWT secret (required)
```

### Load from .env
```bash
export $(cat .env | xargs)
make run
```

### Example .env
```
DB_PATH=/var/lib/drone-app/app.db
GRPC_ADDRESS=:50051
JWT_SECRET=your-strong-secret-key
```

---

## 📁 Project Structure

```
cmd/server/main.go           Entry point (uses config package)
internal/
├── auth/                    JWT authentication
├── config/                  Configuration management
├── db/                      Database & migrations
├── geo/                     Geolocation utilities
└── grpc/                    gRPC services
models/                      Domain entities
repository/                  Data access layer
api/                         gRPC definitions
```

---

## 🔑 Key Files

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | Application entry point |
| `internal/config/config.go` | Configuration management |
| `internal/auth/jwt.go` | JWT validation |
| `internal/geo/distance.go` | Geolocation math |
| `repository/interfaces.go` | Repository contracts |
| `Makefile` | Build automation |
| `Dockerfile` | Container image |
| `README.md` | Full documentation |

---

## 🛠️ Development

### Format Code
```bash
make fmt                     # Format with gofmt
```

### Lint Code
```bash
make lint                    # Run golangci-lint
make vet                     # Run go vet
```

### Generate Proto
```bash
make proto                   # Generate from .proto files
```

### Clean Build
```bash
make clean                   # Remove artifacts
```

---

## 📦 Dependencies

### Build Requirements
- Go 1.21+
- SQLite3 (bundled)

### Development Requirements
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Install all:
```bash
make install-tools
```

---

## 🚀 Deployment

### Docker Build & Run
```bash
make docker-build
docker run \
  -e JWT_SECRET="prod-secret" \
  -e DB_PATH=/var/lib/drone-app/app.db \
  -p 50051:50051 \
  -v drone-db:/var/lib/drone-app \
  drone-app:latest
```

### Kubernetes
```bash
kubectl create secret generic drone-app-secrets --from-literal=jwt-secret="your-secret"
kubectl apply -f k8s-deployment.yaml
```

---

## 🔒 Security Checklist

### Before Production
- [ ] Generate strong JWT_SECRET (e.g., `openssl rand -base64 32`)
- [ ] Use persistent database path
- [ ] Configure TLS for gRPC
- [ ] Use secrets management (AWS Secrets Manager, Vault)
- [ ] Enable monitoring and logging
- [ ] Test backup/restore procedures
- [ ] Review firewall rules
- [ ] Enable rate limiting

### Environment
```bash
# Good
JWT_SECRET=$(openssl rand -base64 32)
DB_PATH=/var/lib/drone-app/app.db
GRPC_ADDRESS=:50051

# Bad
JWT_SECRET=dev-secret          # Too simple
DB_PATH=/tmp/app.db            # Not persistent
GRPC_ADDRESS=0.0.0.0:50051    # Exposes publicly
```

---

## 🐛 Troubleshooting

### Build Issues
```bash
# Clear Go cache
go clean -cache

# Check Go version
go version

# Verify dependencies
go mod verify
```

### Database Lock
```bash
# Check processes
lsof /path/to/app.db

# Ensure single instance running
ps aux | grep drone-app
```

### Authentication Errors
```bash
# Verify JWT secret matches
JWT_SECRET="your-secret" ./drone-app

# Check token validity
# Token must have: name, kind (admin/enduser/drone)
```

### Docker Issues
```bash
# Check image size
docker images drone-app:latest

# View container logs
docker logs <container-id>

# Exec into container
docker exec -it <container-id> sh
```

---

## 📊 Makefile Commands

```bash
make build              # Compile binary
make run               # Build and run
make test              # Run tests
make test-coverage     # Generate coverage report
make coverage-html     # Open HTML coverage
make fmt               # Format code
make lint              # Lint code
make vet               # Vet code
make proto             # Generate protobuf
make clean             # Clean artifacts
make deps              # Download dependencies
make mod-tidy          # Tidy modules
make docker-build      # Build Docker image
make docker-run        # Run Docker container
make check             # Run all checks
make install-tools     # Install dev tools
make help              # Show all commands
```

---

## 🔗 Useful Links

- [Go Documentation](https://golang.org/doc/)
- [gRPC Go Guide](https://grpc.io/docs/languages/go/)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)
- [SQLite Documentation](https://www.sqlite.org/docs.html)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)

---

## 📞 Common Tasks

### Add New Proto Message
1. Edit `.proto` file in `api/*/`
2. Run `make proto`
3. Update corresponding service handler

### Add New Database Migration
1. Create `internal/db/migrations/NNNN_name.up.sql`
2. Create `internal/db/migrations/NNNN_name.down.sql`
3. Restart application (migrations auto-apply)

### Add New Repository Method
1. Add interface in `repository/interfaces.go`
2. Implement in `repository/*_repository.go`
3. Add tests in `repository/*_repository_test.go`

### Add New Configuration
1. Add to `internal/config/config.go` struct
2. Add environment variable in `Load()` function
3. Document in `README.md` and `.env.example`

---

## 📝 Code Standards

### File Naming
- `*_repository.go` - Repository implementations
- `*_server.go` - gRPC service implementations
- `*_test.go` - Test files

### Package Organization
- `internal/` - Private packages
- `api/` - Public interfaces (gRPC)
- `models/` - Domain entities
- `repository/` - Data access

### Naming Conventions
- Packages: lowercase, single word
- Types: PascalCase
- Functions: PascalCase (public), camelCase (private)
- Constants: UPPER_SNAKE_CASE
- Variables: camelCase

### Interfaces
- Name ending with `I` (e.g., `UserRepositoryI`)
- Define in dedicated files (e.g., `interfaces.go`)
- Keep interfaces small and focused

---

## ⚡ Performance Tips

### Build Size
```bash
# Optimized build
go build -ldflags="-s -w" -o drone-app ./cmd/server

# Docker image is already optimized (~30MB)
```

### Database
```bash
# Enable WAL mode (already enabled in db.go)
PRAGMA journal_mode=WAL

# Set busy timeout
PRAGMA busy_timeout=5000

# Enable foreign keys
PRAGMA foreign_keys=ON
```

### Deployment
```bash
# Use persistent volumes for database
docker run -v drone-db:/var/lib/drone-app drone-app:latest

# Replicas for high availability (Kubernetes)
replicas: 2  # Or more for HA
```

---

**Last Updated**: December 17, 2025

