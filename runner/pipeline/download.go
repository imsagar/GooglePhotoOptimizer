// Package pipeline implements the runner's download/encode/upload command
// handlers: the actual work a "download", "encode" or "upload" command from
// the server triggers. Each handler takes a sendStatus callback rather than
// a *ws.Client so it has no dependency on the ws package.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/google"
)

// StoragePath returns cfg.StoragePath/<date>/<subdir>, defaulting to
// ~/GooglePhotosOptimized when StoragePath is unset. Pass "" for date to use today.
func StoragePath(cfg *config.Config, subdir, date string) string {
	base := cfg.StoragePath
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "GooglePhotosOptimized")
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	return filepath.Join(base, date, subdir)
}

// StorageBase returns the root storage directory (no date/subdir).
func StorageBase(cfg *config.Config) string {
	base := cfg.StoragePath
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "GooglePhotosOptimized")
	}
	return base
}

// HandleDownload downloads videoID's original file from Google Photos to
// local storage, reporting progress via sendStatus. Returns the local path
// the file was written to.
func HandleDownload(ctx context.Context, sendStatus func(protocol.Status), photos *google.PhotosClient, cfg *config.Config, videoID string, jobID int, baseURL string, filename string) (string, error) {
	dir := StoragePath(cfg, "originals", "")
	if err := os.MkdirAll(dir, 0755); err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("download: %v", err)})
		return "", err
	}
	localName := filename
	if localName == "" {
		localName = videoID + ".mp4"
	}
	destPath := filepath.Join(dir, localName)

	sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "downloading", Percent: 0})

	var resumeFrom int64
	if info, err := os.Stat(destPath); err == nil {
		resumeFrom = info.Size()
	}

	var lastPct int
	if err := photos.DownloadVideo(ctx, baseURL, destPath, resumeFrom, func(pct int) {
		if pct == 0 || pct == 100 || pct-lastPct >= 3 {
			sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "downloading", Percent: pct})
			lastPct = pct
		}
	}); err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("download: %v", err)})
		return "", err
	}
	return destPath, nil
}
