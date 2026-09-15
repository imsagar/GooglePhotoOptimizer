package ffmpeg

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// DurationMs returns the video duration in milliseconds via ffprobe.
func DurationMs(ctx context.Context, path string) (int, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, err
	}
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}
	return int(secs * 1000), nil
}
