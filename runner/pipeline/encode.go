package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/ffmpeg"
)

// HandleEncode transcodes originalPath with ffmpeg per opts, reporting
// progress via sendStatus and a final job_complete status with the size
// savings. createdTime (RFC3339, may be empty) is the video's real capture
// date; it's embedded in the output and used as its filesystem timestamp.
// Returns the local path of the encoded file.
func HandleEncode(ctx context.Context, sendStatus func(protocol.Status), ffmpegPath string, cfg *config.Config, originalPath string, jobID int, opts ffmpeg.EncodeOpts, durationMs int, createdTime string) (string, error) {
	opts.CreationTime = createdTime
	dir := StoragePath(cfg, "optimized", "")
	if err := os.MkdirAll(dir, 0755); err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}

	base := filepath.Base(originalPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	outputPath := filepath.Join(dir, name+"-o.mp4")

	if durationMs <= 0 {
		if d, err := ffmpeg.DurationMs(ctx, originalPath); err == nil && d > 0 {
			durationMs = d
		}
	}

	// Explicit encode-start event: moves the job off "downloading" and sets
	// downloaded_at server-side even if ffmpeg is slow to emit progress (or
	// emits none, e.g. when duration is unknown).
	sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "encoding", Percent: 0})

	var lastPct int
	err := ffmpeg.Encode(ctx, ffmpegPath, originalPath, outputPath, opts, durationMs, func(pct int) {
		if pct == 0 || pct == 100 || pct-lastPct >= 3 {
			sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "encoding", Percent: pct})
			lastPct = pct
		}
	})
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}

	origInfo, err := os.Stat(originalPath)
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}
	ts := origInfo.ModTime()
	if t, perr := time.Parse(time.RFC3339, createdTime); perr == nil {
		ts = t
	}
	os.Chtimes(outputPath, ts, ts)

	// ffmpeg (Homebrew, un-notarized) taints its output with a macOS
	// quarantine flag, which triggers Gatekeeper's "could not verify" prompt.
	// The file is produced locally and trusted, so strip it. No-op off macOS.
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", outputPath).Run()

	info, err := os.Stat(outputPath)
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}

	sendStatus(protocol.Status{
		Type:          "job_complete",
		JobID:         jobID,
		OriginalSize:  origInfo.Size(),
		OptimizedSize: info.Size(),
	})
	return outputPath, nil
}
