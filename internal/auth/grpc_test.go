package auth

import (
	"context"
	"testing"

	"droneDeliveryManagement/internal/testutil"
	"droneDeliveryManagement/repository"
	"google.golang.org/grpc"
)

func TestRequireKindAndHelpers(t *testing.T) {
	ctx := WithPrincipal(context.Background(), &Principal{Name: "d1", Kind: "drone"})
	if _, err := RequireDrone(ctx); err != nil {
		t.Fatalf("RequireDrone: %v", err)
	}
	if _, err := RequireEndUserOrAdmin(ctx); err == nil {
		t.Fatalf("expected enduser/admin rejection for drone")
	}
}

func TestRequireAdmin_WithDBRoleCheck(t *testing.T) {
	d := testutil.OpenInMemoryDB(t, "authadmin")
	users := repository.NewUserRepository(d)
	// Seed end user alice
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := users.Create(ctx, "alice"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	// Spoofed principal kind=admin but DB role is end user
	pctx := WithPrincipal(context.Background(), &Principal{Name: "alice", Kind: "admin"})
	if _, err := RequireAdmin(pctx, users); err == nil {
		t.Fatalf("expected PermissionDenied for non-admin role")
	}

	// Make real admin
	if err := users.UpdateRoleByUsername(ctx, "alice", "admin"); err != nil {
		t.Fatalf("update role: %v", err)
	}
	if _, err := RequireAdmin(pctx, users); err != nil {
		t.Fatalf("RequireAdmin real admin: %v", err)
	}
}

func TestUnaryAuthInterceptor(t *testing.T) {
	secret := "s3cr3t"
	// allowlisted method should bypass auth
	interceptor := NewUnaryAuthInterceptor(secret, "/health")

	// 1) Allowlisted path: no header -> handler executes, no principal
	hCalled := false
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/health"}, func(ctx context.Context, req any) (any, error) {
		hCalled = true
		if p, ok := FromContext(ctx); ok && p != nil {
			t.Fatalf("expected no principal on allowlisted path")
		}
		return 123, nil
	})
	if err != nil || !hCalled {
		t.Fatalf("allowlisted handler err=%v called=%v", err, hCalled)
	}

	// 2) Authenticated path: with token -> principal injected
	tok := testutil.GenerateJWTHS256(t, secret, "bob", "enduser")
	ctx := testutil.CtxWithBearer(context.Background(), tok)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Op"}, func(ctx context.Context, req any) (any, error) {
		p, ok := FromContext(ctx)
		if !ok || p == nil || p.Name != "bob" || p.Kind != "enduser" {
			t.Fatalf("principal not injected: %+v ok=%v", p, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor auth path: %v", err)
	}
}
