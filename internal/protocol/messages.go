// Package protocol defines the shared message types exchanged between the
// server and the local runner over the WebSocket connection.
package protocol

import "encoding/json"

// Command is sent Server → Runner.
type Command struct {
	Type           string   `json:"type"`
	VideoID        string   `json:"video_id,omitempty"`
	JobID          int      `json:"job_id,omitempty"`
	Codec          string   `json:"codec,omitempty"`
	CRF            int      `json:"crf,omitempty"`
	Preset         string   `json:"preset,omitempty"`
	DeleteOriginal bool     `json:"delete_original,omitempty"`
	Targets        []string `json:"targets,omitempty"` // for delete_local
}

// Status is sent Runner → Server.
type Status struct {
	Type             string          `json:"type"`
	Platform         string          `json:"platform,omitempty"`
	FFmpegVersion    string          `json:"ffmpeg_version,omitempty"`
	JobID            int             `json:"job_id,omitempty"`
	Stage            string          `json:"stage,omitempty"`
	Percent          int             `json:"percent,omitempty"`
	ETASeconds       int             `json:"eta_seconds,omitempty"`
	OriginalSize     int64           `json:"original_size,omitempty"`
	OptimizedSize    int64           `json:"optimized_size,omitempty"`
	OriginalDeleted  bool            `json:"original_deleted,omitempty"`
	SizeVerified     bool            `json:"size_verified,omitempty"`
	DurationVerified bool            `json:"duration_verified,omitempty"`
	Connected        bool            `json:"connected,omitempty"`
	Count            int             `json:"count,omitempty"`
	Videos           json.RawMessage `json:"videos,omitempty"`
	Files            json.RawMessage `json:"files,omitempty"`
	Message          string          `json:"message,omitempty"`
	Stderr           string          `json:"stderr,omitempty"`
}
