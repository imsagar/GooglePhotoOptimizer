package store

import (
	"context"
	"database/sql"
)

// CreateUser upserts a user by email — Google Sign-In returns the same
// account on every login, updating name/avatar in place.
func (db *DB) CreateUser(ctx context.Context, email, name, avatarURL string) (User, error) {
	var u User
	err := db.pool.QueryRowContext(ctx,
		`INSERT INTO users (email, name, avatar_url) VALUES ($1, $2, $3)
         ON CONFLICT (email) DO UPDATE SET name = $2, avatar_url = $3
         RETURNING id, email, name, avatar_url, created_at`,
		email, name, avatarURL,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	return u, err
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := db.pool.QueryRowContext(ctx,
		`SELECT id, email, name, avatar_url, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return u, ErrNotFound
	}
	return u, err
}
