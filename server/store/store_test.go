package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/user/gpoptimizer/server/store"
)

func testDB(t *testing.T) *store.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://gpopt:gpopt@localhost:5432/gpoptimizer?sslmode=disable"
	}
	db, err := store.New(url)
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndGetUser(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, err := db.CreateUser(ctx, "test@example.com", "Test User", "https://example.com/avatar.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "test@example.com" {
		t.Fatalf("expected test@example.com, got %s", u.Email)
	}

	got, err := db.GetUserByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatalf("expected ID %s, got %s", u.ID, got.ID)
	}

	// cleanup
	db.Pool().ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
}

func TestCreatePairingCodeAndValidate(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, _ := db.CreateUser(ctx, "pair@test.com", "Pair", "")
	defer db.Pool().ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	pc, err := db.CreatePairingCode(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pc.Code) != 6 {
		t.Fatalf("expected 6-char code, got %q", pc.Code)
	}

	validated, err := db.ValidatePairingCode(ctx, pc.Code)
	if err != nil {
		t.Fatal(err)
	}
	if validated.UserID != u.ID {
		t.Fatal("wrong user ID")
	}

	// second use should fail
	_, err = db.ValidatePairingCode(ctx, pc.Code)
	if err == nil {
		t.Fatal("expected error on reuse")
	}
}
