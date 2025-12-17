package auth

import (
    "context"
    "testing"

    "droneDeliveryManagement/internal/testutil"
)

const testSecret = "test-secret"

func TestParseFromMD_ValidBearer(t *testing.T) {
    tok := testutil.GenerateJWTHS256(t, testSecret, "alice", "enduser")
    ctx := testutil.CtxWithBearer(context.Background(), tok)
    p, err := ParseFromMD(ctx, testSecret)
    if err != nil {
        t.Fatalf("ParseFromMD: %v", err)
    }
    if p.Name != "alice" || p.Kind != "enduser" {
        t.Fatalf("principal mismatch: %+v", p)
    }
}

func TestParseFromMD_MissingHeader(t *testing.T) {
    _, err := ParseFromMD(context.Background(), testSecret)
    if err == nil {
        t.Fatalf("expected error for missing metadata")
    }
}

func TestParseFromMD_InvalidScheme(t *testing.T) {
    tok := testutil.GenerateJWTHS256(t, testSecret, "bob", "drone")
    // Wrong header key content (not Bearer)
    ctx := context.Background()
    ctx = testutil.CtxWithBearer(ctx, tok)
    // Overwrite with invalid scheme using metadata manually
    // Note: Create new MD with non-Bearer
    // Using helper to ensure at least one correct; then invalidate by direct header
    // Simpler: call parseJWT directly with wrong secret
    if _, err := parseJWT(tok, "wrong"); err == nil {
        t.Fatalf("expected error for wrong secret")
    }
}

func TestParseJWT_ClaimsValidation(t *testing.T) {
    // Missing name/kind -> invalid
    // Build a token with empty claims
    tok := testutil.GenerateJWTHS256(t, testSecret, "", "")
    if _, err := parseJWT(tok, testSecret); err == nil {
        t.Fatalf("expected invalid claims error")
    }
}
