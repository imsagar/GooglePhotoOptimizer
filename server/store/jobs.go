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
	sb.WriteString(`INSERT INTO jobs (user_id, video_id, filename, codec, crf, preset, run_date) VALUES `)

	args := make([]interface{}, 0, len(params)*7)
	for i, p := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		base := i * 7
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7)
		args = append(args, userID, p.VideoID, p.Filename, p.Codec, p.CRF, p.Preset, p.RunDate)
	}
	sb.WriteString(` RETURNING id, user_id, video_id, filename, status, run_date, original_size, optimized_size,
        savings_pct, codec, crf, preset, drive_file_id, error, progress, delete_original, size_verified,
        duration_verified, downloaded_at, optimized_at, uploaded_at, created_at, updated_at`)

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
	var filenameStr, errStr, driveFileID sql.NullString
	var origSize, optSize sql.NullInt64
	var savPct sql.NullFloat64
	var downloadedAt, optimizedAt, uploadedAt sql.NullTime
	err := s.Scan(&j.ID, &j.UserID, &j.VideoID, &filenameStr, &j.Status, &j.RunDate, &origSize, &optSize,
		&savPct, &j.Codec, &j.CRF, &j.Preset, &driveFileID, &errStr, &j.Progress, &j.DeleteOriginal, &j.SizeVerified,
		&j.DurationVerified, &downloadedAt, &optimizedAt, &uploadedAt, &j.CreatedAt, &j.UpdatedAt)
	j.Filename = filenameStr.String
	j.DriveFileID = driveFileID.String
	j.Error = errStr.String
	j.OriginalSize = origSize.Int64
	j.OptimizedSize = optSize.Int64
	j.SavingsPct = float32(savPct.Float64)
	if downloadedAt.Valid {
		j.DownloadedAt = &downloadedAt.Time
	}
	if optimizedAt.Valid {
		j.OptimizedAt = &optimizedAt.Time
	}
	if uploadedAt.Valid {
		j.UploadedAt = &uploadedAt.Time
	}
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

	query := fmt.Sprintf(`SELECT id, user_id, video_id, filename, status, run_date, original_size, optimized_size,
        savings_pct, codec, crf, preset, drive_file_id, error, progress, delete_original, size_verified,
        duration_verified, downloaded_at, optimized_at, uploaded_at, created_at, updated_at, COUNT(*) OVER() AS total
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
		var filenameStr, errStr, driveFileID sql.NullString
		var origSize, optSize sql.NullInt64
		var savPct sql.NullFloat64
		var downloadedAt, optimizedAt, uploadedAt sql.NullTime
		if err := rows.Scan(&j.ID, &j.UserID, &j.VideoID, &filenameStr, &j.Status, &j.RunDate, &origSize, &optSize,
			&savPct, &j.Codec, &j.CRF, &j.Preset, &driveFileID, &errStr, &j.Progress, &j.DeleteOriginal, &j.SizeVerified,
			&j.DurationVerified, &downloadedAt, &optimizedAt, &uploadedAt, &j.CreatedAt, &j.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		j.Filename = filenameStr.String
		j.DriveFileID = driveFileID.String
		j.Error = errStr.String
		j.OriginalSize = origSize.Int64
		j.OptimizedSize = optSize.Int64
		j.SavingsPct = float32(savPct.Float64)
		if downloadedAt.Valid {
			j.DownloadedAt = &downloadedAt.Time
		}
		if optimizedAt.Valid {
			j.OptimizedAt = &optimizedAt.Time
		}
		if uploadedAt.Valid {
			j.UploadedAt = &uploadedAt.Time
		}
		jobs = append(jobs, j)
	}
	return jobs, total, rows.Err()
}

func (db *DB) GetJob(ctx context.Context, jobID int, userID uuid.UUID) (Job, error) {
	row := db.pool.QueryRowContext(ctx,
		`SELECT id, user_id, video_id, filename, status, run_date, original_size, optimized_size,
                savings_pct, codec, crf, preset, drive_file_id, error, progress, delete_original, size_verified,
                duration_verified, downloaded_at, optimized_at, uploaded_at, created_at, updated_at
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

func (db *DB) UpdateJobError(ctx context.Context, jobID int, userID uuid.UUID, message string) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET status = 'failed', progress = 0, error = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`,
		message, jobID, userID,
	)
	return err
}

func (db *DB) UpdateJobDriveFileID(ctx context.Context, jobID int, userID uuid.UUID, driveFileID string) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET drive_file_id = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`,
		driveFileID, jobID, userID,
	)
	return err
}

func (db *DB) DeleteJobs(ctx context.Context, userID uuid.UUID, status string) (int64, error) {
	where := "user_id = $1"
	args := []interface{}{userID}
	if status != "" {
		where += " AND status = $2"
		args = append(args, status)
	}
	res, err := db.pool.ExecContext(ctx, "DELETE FROM jobs WHERE "+where, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (db *DB) ResetJobForReEncode(ctx context.Context, jobID int, userID uuid.UUID, codec string, crf int, preset string) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET status = 'encoding', progress = 0, codec = $1, crf = $2, preset = $3,
		 original_size = 0, optimized_size = 0, savings_pct = 0, error = NULL, updated_at = NOW()
		 WHERE id = $4 AND user_id = $5`,
		codec, crf, preset, jobID, userID,
	)
	return err
}

func (db *DB) UpdateJobResult(ctx context.Context, jobID int, userID uuid.UUID, originalSize, optimizedSize int64, savingsPct float32) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET original_size = $1, optimized_size = $2, savings_pct = $3, optimized_at = NOW(), updated_at = NOW() WHERE id = $4 AND user_id = $5`,
		originalSize, optimizedSize, savingsPct, jobID, userID,
	)
	return err
}

func (db *DB) SetJobDownloadedAt(ctx context.Context, jobID int, userID uuid.UUID) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET downloaded_at = NOW(), updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		jobID, userID,
	)
	return err
}

func (db *DB) SetJobUploadedAt(ctx context.Context, jobID int, userID uuid.UUID) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE jobs SET uploaded_at = NOW(), updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		jobID, userID,
	)
	return err
}
