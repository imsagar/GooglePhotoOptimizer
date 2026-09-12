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

// storagePath returns cfg.StoragePath/<today>/<subdir>, defaulting to
// ~/GooglePhotosOptimized when StoragePath is unset.
func storagePath(cfg *config.Config, subdir string) string {
	base := cfg.StoragePath
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "GooglePhotosOptimized")
	}
	runDate := time.Now().Format("2006-01-02")
	return filepath.Join(base, runDate, subdir)
}

// HandleDownload downloads videoID's original file from Google Photos to
// local storage, reporting progress via sendStatus. Returns the local path
// the file was written to.
func HandleDownload(ctx context.Context, sendStatus func(protocol.Status), photos *google.PhotosClient, cfg *config.Config, videoID string, jobID int, baseURL string) (string, error) {
	dir := storagePath(cfg, "originals")
	if err := os.MkdirAll(dir, 0755); err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("download: %v", err)})
		return "", err
	}
	destPath := filepath.Join(dir, videoID+".mp4")

	sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "downloading", Percent: 0})

	var resumeFrom int64
	if info, err := os.Stat(destPath); err == nil {
		resumeFrom = info.Size()
	}

	if err := photos.DownloadVideo(ctx, baseURL, destPath, resumeFrom); err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("download: %v", err)})
		return "", err
	}

	sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "downloading", Percent: 100})
	return destPath, nil
}
