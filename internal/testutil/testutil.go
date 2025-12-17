package testutil

import (
	"context"
	"database/sql"
	"testing"

	jwt "github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"

	"droneDeliveryManagement/internal/db"
)

// OpenInMemoryDB opens an in-memory SQLite database and applies migrations.
// Caller is responsible for closing the DB, typically via t.Cleanup.
func OpenInMemoryDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	// We use a shared cache memory database so that multiple connections share the same DB if needed.
	d, err := db.Open("file:" + name + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// GenerateJWTHS256 returns a signed JWT string with minimal claims used by the app.
func GenerateJWTHS256(t *testing.T, secret, name, kind string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"name": name,
		"kind": kind,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

// CtxWithBearer returns a context containing gRPC metadata Authorization header with the given token.
func CtxWithBearer(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewIncomingContext(ctx, md)
}
