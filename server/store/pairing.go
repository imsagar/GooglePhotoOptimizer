package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const pairingCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I
const pairingCodeLen = 6
const pairingCodeTTL = 10 * time.Minute

func randomPairingCode() (string, error) {
	buf := make([]byte, pairingCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, pairingCodeLen)
	for i, b := range buf {
		code[i] = pairingCodeChars[int(b)%len(pairingCodeChars)]
	}
	return string(code), nil
}

// CreatePairingCode generates a 6-char alphanumeric code valid for 10 minutes,
// retrying on the rare collision with an existing code.
func (db *DB) CreatePairingCode(ctx context.Context, userID uuid.UUID) (PairingCode, error) {
	var pc PairingCode
	for attempt := 0; attempt < 5; attempt++ {
		code, err := randomPairingCode()
		if err != nil {
			return pc, err
		}
		expiresAt := time.Now().Add(pairingCodeTTL)
		err = db.pool.QueryRowContext(ctx,
			`INSERT INTO pairing_codes (code, user_id, expires_at) VALUES ($1, $2, $3)
             RETURNING code, user_id, expires_at, used`,
			code, userID, expiresAt,
		).Scan(&pc.Code, &pc.UserID, &pc.ExpiresAt, &pc.Used)
		if err == nil {
			return pc, nil
		}
		// retry on primary-key collision, otherwise bail out
		if !isUniqueViolation(err) {
			return pc, err
		}
	}
	return pc, errors.New("could not generate unique pairing code")
}

// ValidatePairingCode checks the code is unused and unexpired, then marks it used.
func (db *DB) ValidatePairingCode(ctx context.Context, code string) (PairingCode, error) {
	tx, err := db.pool.BeginTx(ctx, nil)
	if err != nil {
		return PairingCode{}, err
	}
	defer tx.Rollback()

	var pc PairingCode
	err = tx.QueryRowContext(ctx,
		`SELECT code, user_id, expires_at, used FROM pairing_codes WHERE code = $1 FOR UPDATE`,
		code,
	).Scan(&pc.Code, &pc.UserID, &pc.ExpiresAt, &pc.Used)
	if err == sql.ErrNoRows {
		return pc, ErrNotFound
	}
	if err != nil {
		return pc, err
	}
	if pc.Used {
		return pc, errors.New("pairing code already used")
	}
	if time.Now().After(pc.ExpiresAt) {
		return pc, errors.New("pairing code expired")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE pairing_codes SET used = TRUE WHERE code = $1`, code); err != nil {
		return pc, err
	}
	if err := tx.Commit(); err != nil {
		return pc, err
	}
	pc.Used = true
	return pc, nil
}

func isUniqueViolation(err error) bool {
	// lib/pq reports unique_violation as SQLSTATE 23505.
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
