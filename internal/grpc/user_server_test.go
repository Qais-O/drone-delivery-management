//go:build grpcserver

package grpcserver

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	userv1 "droneDeliveryManagement/api/user/v1"
	"droneDeliveryManagement/internal/auth"
	"droneDeliveryManagement/internal/db"
	"droneDeliveryManagement/repository"
)

// newTestDeps opens an in-memory sqlite DB and returns repos and cleanup.
func newTestDeps(t *testing.T) (*repository.UserRepository, *repository.OrderRepository, func()) {
	t.Helper()
	d, err := db.Open("file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return repository.NewUserRepository(d), repository.NewOrderRepository(d), func() { _ = d.Close() }
}

// newPrincipalCtx returns a context with the given principal injected.
func newPrincipalCtx(name, kind string) context.Context {
	p := &auth.Principal{Name: name, Kind: kind}
	return auth.WithPrincipal(context.Background(), p)
}

// createUser ensures a user exists and returns it.
func createUser(t *testing.T, users *repository.UserRepository, username string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := users.Create(ctx, username); err != nil {
		// If exists, it's fine; try to fetch to confirm.
		if u, err2 := users.GetByUsername(ctx, username); err2 != nil || u == nil {
			t.Fatalf("ensure user: create err=%v, get=%v u=%v", err, err2, u)
		}
	}
}

func TestListOrders_PaginationChaining(t *testing.T) {
	users, orders, cleanup := newTestDeps(t)
	defer cleanup()

	username := "alice"
	createUser(t, users, username)

	// Build server instance directly (no network)
	s := &Server{Users: users, Orders: orders}

	// Place 3 orders via SetOrder
	ctx := newPrincipalCtx(username, "enduser")
	for i := 0; i < 3; i++ {
		// slight delay to vary placement_date seconds (not strictly required)
		if i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
		_, err := s.SetOrder(ctx, &userv1.SetOrderRequest{
			Origin:      &userv1.Coordinates{Lat: 1 + float64(i), Lng: 2 + float64(i)},
			Destination: &userv1.Coordinates{Lat: 3 + float64(i), Lng: 4 + float64(i)},
		})
		if err != nil {
			t.Fatalf("SetOrder[%d]: %v", i, err)
		}
	}

	// List with page_size=1, follow next_page_token until exhausted
	var allIDs []int64
	token := ""
	for page := 0; page < 5; page++ { // upper bound guard
		resp, err := s.ListOrders(ctx, &userv1.ListOrdersRequest{PageSize: 1, PageToken: token})
		if err != nil {
			t.Fatalf("ListOrders page=%d: %v", page, err)
		}
		if len(resp.GetOrders()) > 0 {
			allIDs = append(allIDs, resp.GetOrders()[0].GetId())
		}
		if resp.GetNextPageToken() == "" {
			break
		}
		// Ensure token changes (progress) to avoid loops
		if resp.GetNextPageToken() == token {
			t.Fatalf("next_page_token did not advance: %q", token)
		}
		token = resp.GetNextPageToken()
	}

	if len(allIDs) != 3 {
		t.Fatalf("expected 3 orders across pages, got %d (%v)", len(allIDs), allIDs)
	}
	// Ensure IDs are distinct
	seen := map[int64]bool{}
	for _, id := range allIDs {
		if seen[id] {
			t.Fatalf("duplicate id in pagination sequence: %d (all=%v)", id, allIDs)
		}
		seen[id] = true
	}
}

func TestListOrders_InvalidToken(t *testing.T) {
	users, orders, cleanup := newTestDeps(t)
	defer cleanup()

	username := "bob"
	createUser(t, users, username)

	s := &Server{Users: users, Orders: orders}
	ctx := newPrincipalCtx(username, "enduser")

	// Place one order
	if _, err := s.SetOrder(ctx, &userv1.SetOrderRequest{
		Origin:      &userv1.Coordinates{Lat: 10, Lng: 20},
		Destination: &userv1.Coordinates{Lat: 30, Lng: 40},
	}); err != nil {
		t.Fatalf("SetOrder: %v", err)
	}

	// Use an invalid (non-base64) token
	_, err := s.ListOrders(ctx, &userv1.ListOrdersRequest{PageSize: 1, PageToken: "***invalid***"})
	if err == nil {
		t.Fatalf("expected error for invalid token, got nil")
	}
}

func TestWithdrawOrder(t *testing.T) {
	users, orders, cleanup := newTestDeps(t)
	defer cleanup()

	username := "carol"
	createUser(t, users, username)

	s := &Server{Users: users, Orders: orders}
	ctx := newPrincipalCtx(username, "enduser")

	// Place order
	setResp, err := s.SetOrder(ctx, &userv1.SetOrderRequest{
		Origin:      &userv1.Coordinates{Lat: 7, Lng: 8},
		Destination: &userv1.Coordinates{Lat: 9, Lng: 10},
	})
	if err != nil {
		t.Fatalf("SetOrder: %v", err)
	}
	oid := setResp.GetOrder().GetId()

	// Withdraw
	wResp, err := s.WithdrawOrder(ctx, &userv1.WithdrawOrderRequest{OrderId: oid})
	if err != nil {
		t.Fatalf("WithdrawOrder: %v", err)
	}
	if got := wResp.GetOrder().GetStatus(); got != userv1.Status_STATUS_WITHDRAWN {
		t.Fatalf("withdrawn status = %v, want %v", got, userv1.Status_STATUS_WITHDRAWN)
	}

	// List and ensure the order is present and marked withdrawn
	lResp, err := s.ListOrders(ctx, &userv1.ListOrdersRequest{PageSize: 10})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	found := false
	for _, o := range lResp.GetOrders() {
		if o.GetId() == oid {
			found = true
			if o.GetStatus() != userv1.Status_STATUS_WITHDRAWN {
				t.Fatalf("order status after withdraw = %v, want withdrawn", o.GetStatus())
			}
		}
	}
	if !found {
		t.Fatalf("withdrawn order id=%d not found in list", oid)
	}
}

// TestEncodeDecodeCursor_RoundTrip tests cursor encoding and decoding round trip.
func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	sec := int64(1700000000)
	id := int64(42)
	token := encodeCursor(sec, id)
	// token should be URL-safe base64, no padding
	if strings.Contains(token, "=") {
		t.Fatalf("cursor token should be raw base64 url without padding: %q", token)
	}
	if _, err := base64.RawURLEncoding.DecodeString(token); err != nil {
		t.Fatalf("token not valid base64: %v", err)
	}
	var gotSec, gotID int64
	if err := decodeCursor(token, &gotSec, &gotID); err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if gotSec != sec || gotID != id {
		t.Fatalf("round trip mismatch: got (%d,%d) want (%d,%d)", gotSec, gotID, sec, id)
	}
}

// TestDecodeCursor_InvalidFormat tests decoding with invalid formats.
func TestDecodeCursor_InvalidFormat(t *testing.T) {
	var s, i int64
	// not base64
	if err := decodeCursor("***", &s, &i); err == nil {
		t.Fatalf("expected error for non-base64 token")
	}
	// wrong parts
	bad := base64.RawURLEncoding.EncodeToString([]byte("not|number|extra"))
	if err := decodeCursor(bad, &s, &i); err == nil {
		t.Fatalf("expected error for invalid parts")
	}
}

// TestPlacementToUnixSeconds tests placement date parsing.
func TestPlacementToUnixSeconds(t *testing.T) {
	// RFC3339
	sec, err := placementToUnixSeconds("2024-01-02T03:04:05Z")
	if err != nil || sec == 0 {
		t.Fatalf("RFC3339 parse failed: sec=%d err=%v", sec, err)
	}
	// SQLite default format
	if _, err := placementToUnixSeconds("2024-01-02 03:04:05"); err != nil {
		t.Fatalf("sqlite format parse failed: %v", err)
	}
	// Unsupported
	if _, err := placementToUnixSeconds("02/01/2024"); err == nil {
		t.Fatalf("expected error for unsupported format")
	}
}
