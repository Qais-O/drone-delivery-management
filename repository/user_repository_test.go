package repository

import (
    "context"
    "testing"

    "droneDeliveryManagement/internal/db"
)

func TestUserRepository_CRUDAndQueries(t *testing.T) {
    d, err := db.Open("file:userrepo?mode=memory&cache=shared")
    if err != nil {
        t.Fatalf("open db: %v", err)
    }
    t.Cleanup(func() { _ = d.Close() })

    repo := NewUserRepository(d)
    ctx := context.Background()

    // Create
    u, err := repo.Create(ctx, "alice")
    if err != nil {
        t.Fatalf("create: %v", err)
    }
    if u.ID == 0 || u.Username != "alice" || u.Role == "" {
        t.Fatalf("unexpected created user: %+v", u)
    }

    // GetByID
    g, err := repo.GetByID(ctx, u.ID)
    if err != nil || g == nil || g.Username != "alice" {
        t.Fatalf("get by id: %v %+v", err, g)
    }

    // GetByUsername
    g2, err := repo.GetByUsername(ctx, "alice")
    if err != nil || g2 == nil || g2.ID != u.ID {
        t.Fatalf("get by username: %v %+v", err, g2)
    }

    // List
    list, err := repo.List(ctx, 10, 0)
    if err != nil || len(list) == 0 {
        t.Fatalf("list: %v len=%d", err, len(list))
    }

    // UpdateRoleByUsername
    if err := repo.UpdateRoleByUsername(ctx, "alice", "admin"); err != nil {
        t.Fatalf("update role: %v", err)
    }
    g3, _ := repo.GetByUsername(ctx, "alice")
    if g3.Role != "admin" {
        t.Fatalf("role not updated: %+v", g3)
    }

    // Delete
    if err := repo.Delete(ctx, u.ID); err != nil {
        t.Fatalf("delete: %v", err)
    }
    gone, err := repo.GetByID(ctx, u.ID)
    if err != nil || gone != nil {
        t.Fatalf("expected user deleted, got: %+v err=%v", gone, err)
    }
}
