package store_test

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/user/gpoptimizer/server/store"
)

func TestAuthenticateRunner(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, err := db.CreateUser(ctx, "runner-auth@test.com", "Runner Auth", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Pool().ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)

	const token = "s3cr3t-runner-token"
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	r, err := db.CreateRunner(ctx, u.ID, string(hash), "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Pool().ExecContext(ctx, "DELETE FROM runners WHERE id = $1", r.ID)

	got, err := db.AuthenticateRunner(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID {
		t.Fatalf("expected runner %s, got %s", r.ID, got.ID)
	}

	if _, err := db.AuthenticateRunner(ctx, "wrong-token"); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound for bad token, got %v", err)
	}
}
