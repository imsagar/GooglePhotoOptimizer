package pipeline_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/ffmpeg"
	"github.com/user/gpoptimizer/runner/pipeline"
)

// TestHandleEncode exercises the real branch logic: it runs an actual
// ffmpeg encode, then checks the job_complete status carries the expected
// original/optimized sizes and that the returned path exists.
func TestHandleEncode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}

	dir := t.TempDir()
	input := dir + "/original.mp4"
	gen := exec.Command("ffmpeg", "-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=30",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", input)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test input: %v\n%s", err, out)
	}

	cfg := &config.Config{StoragePath: dir}
	var statuses []protocol.Status
	sendStatus := func(s protocol.Status) { statuses = append(statuses, s) }

	outPath, err := pipeline.HandleEncode(context.Background(), sendStatus, "ffmpeg", cfg, input, 42,
		ffmpeg.EncodeOpts{Codec: "libx265", CRF: 30, Preset: "ultrafast"}, 0)
	if err != nil {
		t.Fatalf("HandleEncode: %v", err)
	}

	var complete *protocol.Status
	for i := range statuses {
		if statuses[i].Type == "job_complete" {
			complete = &statuses[i]
		}
	}
	if complete == nil {
		t.Fatalf("expected a job_complete status, got %+v", statuses)
	}
	if complete.JobID != 42 {
		t.Errorf("job_complete JobID = %d, want 42", complete.JobID)
	}
	if complete.OriginalSize == 0 || complete.OptimizedSize == 0 {
		t.Errorf("expected non-zero sizes, got original=%d optimized=%d", complete.OriginalSize, complete.OptimizedSize)
	}
	if _, err := exec.Command("test", "-f", outPath).CombinedOutput(); err != nil {
		t.Errorf("encoded output %s does not exist", outPath)
	}
}
