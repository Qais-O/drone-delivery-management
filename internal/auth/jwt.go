package auth

import (
	"context"
	"errors"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
)

// Principal represents the authenticated caller from JWT.
type Principal struct {
	Name string // could be username or drone name
	Kind string // "admin" | "enduser" | "drone"
}

type principalKey struct{}

// WithPrincipal stores the principal in context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext retrieves the principal from context (if any).
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)
	return p, ok
}

// ParseFromMD extracts and validates a Bearer JWT from gRPC metadata and returns a Principal.
func ParseFromMD(ctx context.Context, secret string) (*Principal, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		vals = md.Get("Authorization")
	}
	if len(vals) == 0 {
		return nil, errors.New("missing authorization")
	}
	parts := strings.SplitN(vals[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("invalid authorization header")
	}
	tokenStr := strings.TrimSpace(parts[1])
	return parseJWT(tokenStr, secret)
}

// parseJWT validates and extracts claims from a JWT token.
func parseJWT(tokenStr string, secret string) (*Principal, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	type claims struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
		jwt.RegisteredClaims
	}

	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		if err == nil {
			err = errors.New("invalid token")
		}
		return nil, err
	}
	c, _ := tok.Claims.(*claims)
	if c == nil || c.Name == "" || c.Kind == "" {
		return nil, errors.New("invalid claims")
	}
	return &Principal{Name: c.Name, Kind: strings.ToLower(c.Kind)}, nil
}
