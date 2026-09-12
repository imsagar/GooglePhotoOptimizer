package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (db *DB) CreateRunner(ctx context.Context, userID uuid.UUID, tokenHash, platform, arch string) (Runner, error) {
	var r Runner
	err := db.pool.QueryRowContext(ctx,
		`INSERT INTO runners (user_id, token_hash, platform, arch)
         VALUES ($1, $2, $3, $4)
         RETURNING id, user_id, token_hash, label, platform, arch, ffmpeg_version,
                   google_connected, last_seen_at, status, created_at`,
		userID, tokenHash, platform, arch,
	).Scan(&r.ID, &r.UserID, &r.TokenHash, &r.Label, &r.Platform, &r.Arch, &r.FFmpegVersion,
		&r.GoogleConnected, &r.LastSeenAt, &r.Status, &r.CreatedAt)
	return r, err
}

func (db *DB) GetRunnerByUserID(ctx context.Context, userID uuid.UUID) (Runner, error) {
	var r Runner
	err := db.pool.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, label, platform, arch, ffmpeg_version,
                google_connected, last_seen_at, status, created_at
         FROM runners WHERE user_id = $1`,
		userID,
	).Scan(&r.ID, &r.UserID, &r.TokenHash, &r.Label, &r.Platform, &r.Arch, &r.FFmpegVersion,
		&r.GoogleConnected, &r.LastSeenAt, &r.Status, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return r, ErrNotFound
	}
	return r, err
}

func (db *DB) UpdateRunnerStatus(ctx context.Context, runnerID uuid.UUID, status string, lastSeenAt time.Time) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE runners SET status = $1, last_seen_at = $2 WHERE id = $3`,
		status, lastSeenAt, runnerID,
	)
	return err
}

// AuthenticateRunner finds the runner whose token hash matches token. Runner
// tokens aren't looked up by ID (the runner has no other way to identify
// itself over the wire), so this bcrypt-compares against every row.
//
// ponytail: O(n) over all runners per connect; fine at this scale, add a
// token-prefix index if the runner table gets large.
func (db *DB) AuthenticateRunner(ctx context.Context, token string) (Runner, error) {
	rows, err := db.pool.QueryContext(ctx,
		`SELECT id, user_id, token_hash, label, platform, arch, ffmpeg_version,
                google_connected, last_seen_at, status, created_at
         FROM runners`,
	)
	if err != nil {
		return Runner{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Runner
		if err := rows.Scan(&r.ID, &r.UserID, &r.TokenHash, &r.Label, &r.Platform, &r.Arch, &r.FFmpegVersion,
			&r.GoogleConnected, &r.LastSeenAt, &r.Status, &r.CreatedAt); err != nil {
			return Runner{}, err
		}
		if bcrypt.CompareHashAndPassword([]byte(r.TokenHash), []byte(token)) == nil {
			return r, nil
		}
	}
	if err := rows.Err(); err != nil {
		return Runner{}, err
	}
	return Runner{}, ErrNotFound
}

func (db *DB) UpdateRunnerTokenHash(ctx context.Context, runnerID uuid.UUID, tokenHash string) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE runners SET token_hash = $1 WHERE id = $2`,
		tokenHash, runnerID,
	)
	return err
}

func (db *DB) SetGoogleConnected(ctx context.Context, runnerID uuid.UUID, connected bool) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE runners SET google_connected = $1 WHERE id = $2`,
		connected, runnerID,
	)
	return err
}
