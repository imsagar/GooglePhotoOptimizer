// Package ffmpeg locates and drives an ffmpeg binary for video transcoding.
package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/user/gpoptimizer/runner/config"
)

// EnsureInstalled locates an ffmpeg binary: first on PATH, then in the
// runner's local bin directory (~/.gpoptimizer/bin).
//
// ponytail: auto-download is skipped. Fetching+extracting a static build
// (tar.xz on Linux, zip on macOS/Windows, from different hosts per platform)
// is real code with real failure modes and no easy way to test here. Ask the
// user to install it instead; add the downloader if manual install turns
// out to be too much friction.
func EnsureInstalled() (string, error) {
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}

	localPath := filepath.Join(config.Dir(), "bin", "ffmpeg")
	if runtime.GOOS == "windows" {
		localPath += ".exe"
	}
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	return "", fmt.Errorf(`ffmpeg not found in PATH or %s

Install FFmpeg and make sure it's on your PATH (or place the binary at the
path above):
  macOS:   brew install ffmpeg
  Linux:   sudo apt install ffmpeg   (or your distro's package manager)
  Windows: winget install ffmpeg     (or https://ffmpeg.org/download.html)`, localPath)
}
