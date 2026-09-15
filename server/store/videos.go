package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UpsertVideos bulk-inserts videos, updating metadata on conflict (id, user_id).
func (db *DB) UpsertVideos(ctx context.Context, userID uuid.UUID, videos []Video) error {
	if len(videos) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO videos (id, user_id, google_drive_id, filename, mime_type,
        size_bytes, duration_ms, width, height, creation_time, album_id, album_title, base_url, synced_at)
        VALUES `)

	args := make([]interface{}, 0, len(videos)*13)
	for i, v := range videos {
		if i > 0 {
			sb.WriteString(", ")
		}
		base := i * 13
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW())",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10, base+11, base+12, base+13)
		args = append(args, v.ID, userID, v.GoogleDriveID, v.Filename, v.MimeType,
			v.SizeBytes, v.DurationMs, v.Width, v.Height, v.CreationTime, v.AlbumID, v.AlbumTitle, v.BaseURL)
	}

	sb.WriteString(` ON CONFLICT (id, user_id) DO UPDATE SET
        google_drive_id = EXCLUDED.google_drive_id,
        filename = EXCLUDED.filename,
        mime_type = EXCLUDED.mime_type,
        size_bytes = EXCLUDED.size_bytes,
        duration_ms = EXCLUDED.duration_ms,
        width = EXCLUDED.width,
        height = EXCLUDED.height,
        creation_time = EXCLUDED.creation_time,
        album_id = EXCLUDED.album_id,
        album_title = EXCLUDED.album_title,
        base_url = EXCLUDED.base_url,
        synced_at = NOW()`)

	_, err := db.pool.ExecContext(ctx, sb.String(), args...)
	return err
}

var videoSortColumns = map[string]string{
	"size":     "v.size_bytes",
	"date":     "v.creation_time",
	"duration": "v.duration_ms",
	"name":     "v.filename",
}

// ListVideos returns videos for a user with dynamic filtering/sorting/pagination,
// plus the total row count matching the filters (ignoring pagination).
func (db *DB) ListVideos(ctx context.Context, userID uuid.UUID, params ListVideosParams) ([]Video, int, error) {
	where := []string{"v.user_id = $1"}
	args := []interface{}{userID}

	joinJobs := params.Status != ""

	if params.MinSize > 0 {
		args = append(args, params.MinSize)
		where = append(where, fmt.Sprintf("v.size_bytes >= $%d", len(args)))
	}
	if params.From != nil {
		args = append(args, *params.From)
		where = append(where, fmt.Sprintf("v.creation_time >= $%d", len(args)))
	}
	if params.To != nil {
		args = append(args, *params.To)
		where = append(where, fmt.Sprintf("v.creation_time <= $%d", len(args)))
	}
	if params.AlbumID != "" {
		args = append(args, params.AlbumID)
		where = append(where, fmt.Sprintf("v.album_id = $%d", len(args)))
	}
	if params.Status != "" {
		args = append(args, params.Status)
		where = append(where, fmt.Sprintf("j.status = $%d", len(args)))
	}

	sortCol, ok := videoSortColumns[params.Sort]
	if !ok {
		sortCol = "v.creation_time"
	}
	order := "DESC"
	if strings.EqualFold(params.Order, "asc") {
		order = "ASC"
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

	from := "FROM videos v"
	if joinJobs {
		from += " JOIN jobs j ON j.video_id = v.id AND j.user_id = v.user_id"
	}

	args = append(args, pageSize, offset)
	query := fmt.Sprintf(`SELECT v.id, v.user_id, v.google_drive_id, v.filename, v.mime_type,
        v.size_bytes, v.duration_ms, v.width, v.height, v.creation_time, v.album_id, v.album_title,
        v.base_url, v.synced_at, COUNT(*) OVER() AS total
        %s
        WHERE %s
        ORDER BY %s %s
        LIMIT $%d OFFSET $%d`,
		from, strings.Join(where, " AND "), sortCol, order, len(args)-1, len(args))

	rows, err := db.pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var videos []Video
	total := 0
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.UserID, &v.GoogleDriveID, &v.Filename, &v.MimeType,
			&v.SizeBytes, &v.DurationMs, &v.Width, &v.Height, &v.CreationTime, &v.AlbumID, &v.AlbumTitle,
			&v.BaseURL, &v.SyncedAt, &total); err != nil {
			return nil, 0, err
		}
		videos = append(videos, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (db *DB) DeleteAllVideos(ctx context.Context, userID uuid.UUID) (int64, error) {
	res, err := db.pool.ExecContext(ctx, "DELETE FROM videos WHERE user_id = $1", userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type VideoMeta struct {
	Filename    string
	CreatedTime *time.Time
}

func (db *DB) GetVideoMeta(ctx context.Context, userID uuid.UUID, videoID string) (VideoMeta, error) {
	var m VideoMeta
	err := db.pool.QueryRowContext(ctx,
		`SELECT filename, creation_time FROM videos WHERE id = $1 AND user_id = $2`, videoID, userID,
	).Scan(&m.Filename, &m.CreatedTime)
	return m, err
}

func (db *DB) GetVideoBaseURL(ctx context.Context, userID uuid.UUID, videoID string) (string, error) {
	var baseURL string
	err := db.pool.QueryRowContext(ctx,
		`SELECT COALESCE(base_url, '') FROM videos WHERE id = $1 AND user_id = $2`, videoID, userID,
	).Scan(&baseURL)
	return baseURL, err
}
