package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// ErrNotFound is returned by store lookups that find no matching row.
var ErrNotFound = errors.New("not found")

type DB struct {
	pool *sql.DB
}

func New(databaseURL string) (*DB, error) {
	pool, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)
	if err := pool.Ping(); err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Close() error { return db.pool.Close() }

// Pool exposes the underlying *sql.DB, mainly for test cleanup.
func (db *DB) Pool() *sql.DB { return db.pool }

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Runner struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	TokenHash       string     `json:"-"`
	Label           string     `json:"label"`
	Platform        string     `json:"platform"`
	Arch            string     `json:"arch"`
	FFmpegVersion   string     `json:"ffmpeg_version"`
	GoogleConnected bool       `json:"google_connected"`
	LastSeenAt      *time.Time `json:"last_seen_at"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Video struct {
	ID            string     `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	GoogleDriveID string     `json:"google_drive_id,omitempty"`
	Filename      string     `json:"filename"`
	MimeType      string     `json:"mime_type"`
	SizeBytes     int64      `json:"size_bytes"`
	DurationMs    int        `json:"duration_ms"`
	Width         int        `json:"width"`
	Height        int        `json:"height"`
	CreationTime  *time.Time `json:"creation_time"`
	AlbumID       string     `json:"album_id,omitempty"`
	AlbumTitle    string     `json:"album_title,omitempty"`
	BaseURL       string     `json:"base_url,omitempty"`
	SyncedAt      time.Time  `json:"synced_at"`
}

type Job struct {
	ID               int       `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	VideoID          string    `json:"video_id"`
	Filename         string    `json:"filename,omitempty"`
	Status           string    `json:"status"`
	RunDate          string    `json:"run_date"`
	OriginalSize     int64     `json:"original_size"`
	OptimizedSize    int64     `json:"optimized_size"`
	SavingsPct       float32   `json:"savings_pct"`
	Codec            string    `json:"codec"`
	CRF              int       `json:"crf"`
	Preset           string    `json:"preset"`
	DriveFileID      string    `json:"drive_file_id,omitempty"`
	Error            string    `json:"error,omitempty"`
	Progress         int       `json:"progress"`
	DeleteOriginal   bool      `json:"delete_original"`
	SizeVerified     *bool      `json:"size_verified"`
	DurationVerified *bool      `json:"duration_verified"`
	DownloadedAt     *time.Time `json:"downloaded_at,omitempty"`
	OptimizedAt      *time.Time `json:"optimized_at,omitempty"`
	UploadedAt       *time.Time `json:"uploaded_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type PairingCode struct {
	Code      string    `json:"code"`
	UserID    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

type ListVideosParams struct {
	Sort     string // "size", "date", "duration", "name"
	Order    string // "asc", "desc"
	Page     int
	PageSize int
	MinSize  int64
	From     *time.Time
	To       *time.Time
	AlbumID  string
	Status   string // filter by job status
}

type ListJobsParams struct {
	Status   string
	Page     int
	PageSize int
}

type CreateJobParams struct {
	VideoID  string
	Filename string
	Codec    string
	CRF      int
	Preset   string
	RunDate  string
}
