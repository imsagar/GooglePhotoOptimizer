package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/ffmpeg"
)

// HandleEncode transcodes originalPath with ffmpeg per opts, reporting
// progress via sendStatus and a final job_complete status with the size
// savings. Returns the local path of the encoded file.
func HandleEncode(ctx context.Context, sendStatus func(protocol.Status), ffmpegPath string, cfg *config.Config, originalPath string, jobID int, opts ffmpeg.EncodeOpts, durationMs int) (string, error) {
	dir := StoragePath(cfg, "optimized", "")
	if err := os.MkdirAll(dir, 0755); err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}

	base := filepath.Base(originalPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	outputPath := filepath.Join(dir, name+"-o.mp4")

	err := ffmpeg.Encode(ctx, ffmpegPath, originalPath, outputPath, opts, durationMs, func(pct int) {
		sendStatus(protocol.Status{Type: "progress", JobID: jobID, Stage: "encoding", Percent: pct})
	})
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		sendStatus(protocol.Status{Type: "error", JobID: jobID, Message: fmt.Sprintf("encode: %v", err)})
		return "", err
	}
	origInfo, err := os.Stat(originalPath)
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
