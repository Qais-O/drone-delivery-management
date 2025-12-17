package auth

import (
	"context"
	"strings"

	"droneDeliveryManagement/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewUnaryAuthInterceptor returns a gRPC unary interceptor that extracts and validates
// a Bearer JWT from incoming metadata and injects the Principal into the context.
// Methods listed in allowUnauthenticated will bypass authentication (e.g., health checks).
func NewUnaryAuthInterceptor(secret string, allowUnauthenticated ...string) grpc.UnaryServerInterceptor {
	allow := make(map[string]struct{}, len(allowUnauthenticated))
	for _, m := range allowUnauthenticated {
		allow[strings.TrimSpace(m)] = struct{}{}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := allow[info.FullMethod]; ok {
			return handler(ctx, req)
		}
		p, err := ParseFromMD(ctx, secret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "auth error: %v", err)
		}
		return handler(WithPrincipal(ctx, p), req)
	}
}

// RequirePrincipal ensures a principal is present in context.
func RequirePrincipal(ctx context.Context) (*Principal, error) {
	p, ok := FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	return p, nil
}

// RequireKind ensures the principal has the given kind (lowercased compare).
func RequireKind(ctx context.Context, kind string) (*Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Kind != strings.ToLower(kind) {
		return nil, status.Errorf(codes.PermissionDenied, "only %s can perform this action", strings.ToLower(kind))
	}
	return p, nil
}

// RequireDrone ensures the caller is a drone.
func RequireDrone(ctx context.Context) (*Principal, error) {
	return RequireKind(ctx, "drone")
}

// RequireEndUserOrAdmin ensures the caller is an end user or admin.
func RequireEndUserOrAdmin(ctx context.Context) (*Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Kind != "enduser" && p.Kind != "admin" {
		return nil, status.Error(codes.PermissionDenied, "only enduser or admin can perform this action")
	}
	return p, nil
}

// RequireAdmin ensures the caller is an admin principal AND that the underlying
// user exists with role 'admin'. This prevents spoofing by a non-admin.
func RequireAdmin(ctx context.Context, users *repository.UserRepository) (*Principal, error) {
	p, err := RequireKind(ctx, "admin")
	if err != nil {
		return nil, err
	}
	if users == nil {
		return nil, status.Error(codes.Internal, "users repository not configured")
	}
	u, err := users.GetByUsername(ctx, p.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	if u == nil || strings.ToLower(strings.TrimSpace(u.Role)) != "admin" {
		return nil, status.Error(codes.PermissionDenied, "only admin can perform this action")
	}
	return p, nil
}
