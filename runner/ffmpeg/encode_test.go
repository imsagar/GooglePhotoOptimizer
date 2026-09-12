package ffmpeg_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/user/gpoptimizer/runner/ffmpeg"
)

func TestEncodeProgressParsing(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}

	input := t.TempDir() + "/test_input.mp4"
	output := t.TempDir() + "/test_output.mp4"

	gen := exec.Command("ffmpeg", "-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=30",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", input)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test input: %v\n%s", err, out)
	}

	var lastPct int
	err := ffmpeg.Encode(context.Background(), "ffmpeg", input, output, ffmpeg.EncodeOpts{
		Codec: "libx265", CRF: 28, Preset: "ultrafast",
	}, 2000, func(pct int) {
		lastPct = pct
	})

	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if lastPct < 90 {
		t.Fatalf("expected progress near 100%%, got %d%%", lastPct)
	}
}
