package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CreateJobs batch-inserts jobs for a user and returns the created rows.
func (db *DB) CreateJobs(ctx context.Context, userID uuid.UUID, params []CreateJobParams) ([]Job, error) {
	if len(params) == 0 {
		return nil, nil
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO jobs (user_id, video_id, codec, crf, preset, run_date) VALUES `)

	args := make([]interface{}, 0, len(params)*6)
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		base := i * 6
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6)
		args = append(args, userID, p.VideoID, p.Codec, p.CRF, p.Preset, p.RunDate)
	}
	sb.WriteString(` RETURNING id, user_id, video_id, status, run_date, original_size, optimized_size,
        savings_pct, codec, crf, preset, error, progress, delete_original, size_verified,
        duration_verified, created_at, updated_at`)

	rows, err := db.pool.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanJob(s rowScanner) (Job, error) {
	var j Job
	var errStr sql.NullString
	err := s.Scan(&j.ID, &j.UserID, &j.VideoID, &j.Status, &j.RunDate, &j.OriginalSize, &j.OptimizedSize,
		&j.SavingsPct, &j.Codec, &j.CRF, &j.Preset, &errStr, &j.Progress, &j.DeleteOriginal, &j.SizeVerified,
		&j.DurationVerified, &j.CreatedAt, &j.UpdatedAt)
	j.Error = errStr.String
	return j, err
}

func (db *DB) ListJobs(ctx context.Context, userID uuid.UUID, params ListJobsParams) ([]Job, int, error) {
	where := []string{"user_id = $1"}
	args := []interface{}{userID}

	if params.Status != "" {
		args = append(args, params.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}

	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := fmt.Sprintf(`SELECT id, user_id, video_id, status, run_date, original_size, optimized_size,
        savings_pct, codec, crf, preset, error, progress, delete_original, size_verified,
        duration_verified, created_at, updated_at, COUNT(*) OVER() AS total
        FROM jobs
        WHERE %s
        ORDER BY created_at DESC
        LIMIT $%d OFFSET $%d`, strings.Join(where, " AND "), len(args)-1, len(args))

	rows, err := db.pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []Job
	total := 0
	for rows.Next() {
		var j Job
		var errStr sql.NullString
		if err := rows.Scan(&j.ID, &j.UserID, &j.VideoID, &j.Status, &j.RunDate, &j.OriginalSize, &j.OptimizedSize,
			&j.SavingsPct, &j.Codec, &j.CRF, &j.Preset, &errStr, &j.Progress, &j.DeleteOriginal, &j.SizeVerified,
			&j.DurationVerified, &j.CreatedAt, &j.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		j.Error = errStr.String
		jobs = append(jobs, j)
	}
	return jobs, total, rows.Err()
}

func (db *DB) GetJob(ctx context.Context, jobID int, userID uuid.UUID) (Job, error) {
	row := db.pool.QueryRowContext(ctx,
		`SELECT id, user_id, video_id, status, run_date, original_size, optimized_size,
                savings_pct, codec, crf, preset, error, progress, delete_original, size_verified,
                duration_verified, created_at, updated_at
         FROM jobs WHERE id = $1 AND user_id = $2`,
		jobID, userID,
	)
	j, err := scanJob(row)
	if err == sql.ErrNoRows {
		return j, ErrNotFound
	}
	return j, err
}

func (db *DB) UpdateJobStatus(ctx context.Context, jobID int, userID uuid.UUID, status string, progress int) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET status = $1, progress = $2, updated_at = NOW() WHERE id = $3 AND user_id = $4`,
		status, progress, jobID, userID,
	)
	return err
}

func (db *DB) UpdateJobResult(ctx context.Context, jobID int, userID uuid.UUID, optimizedSize int64, savingsPct float32) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET optimized_size = $1, savings_pct = $2, updated_at = NOW() WHERE id = $3 AND user_id = $4`,
		optimizedSize, savingsPct, jobID, userID,
	)
	return err
}
