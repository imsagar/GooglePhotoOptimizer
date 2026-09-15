package ffmpeg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// EncodeOpts configures the ffmpeg video encode.
type EncodeOpts struct {
	Codec        string // libx265, libx264, libsvtav1
	CRF          int
	Preset       string // ultrafast..veryslow
	CreationTime string // RFC3339 capture date to embed; empty to skip
}

// Encode transcodes input to output with ffmpeg, calling progressFn with a
// 0-100 percentage as ffmpeg reports its progress. totalDurationMs is the
// video's known duration, used to turn ffmpeg's out_time_us into a percent.
func Encode(ctx context.Context, ffmpegPath, input, output string, opts EncodeOpts, totalDurationMs int, progressFn func(pct int)) error {
	args := []string{
		"-i", input,
		"-map_metadata", "0",
	}
	if opts.CreationTime != "" {
		args = append(args, "-metadata", "creation_time="+opts.CreationTime)
	}
	args = append(args,
		"-c:v", opts.Codec,
		"-crf", strconv.Itoa(opts.CRF),
		"-preset", opts.Preset,
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-progress", "pipe:1",
		"-y",
		output,
	)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		usStr, ok := strings.CutPrefix(line, "out_time_us=")
		if !ok || totalDurationMs <= 0 {
			continue
		}
		us, err := strconv.ParseInt(usStr, 10, 64)
		if err != nil {
			continue
		}
		pct := int(us / 1000 * 100 / int64(totalDurationMs))
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
		progressFn(pct)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
