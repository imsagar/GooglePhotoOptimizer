// Command runner is the local GPOptimizer agent: pairs with the server and
// connects over WebSocket to transcode videos.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"golang.org/x/oauth2"
	oauthgoogle "golang.org/x/oauth2/google"

	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/ffmpeg"
	"github.com/user/gpoptimizer/runner/google"
	"github.com/user/gpoptimizer/runner/pairing"
	"github.com/user/gpoptimizer/runner/pipeline"
	"github.com/user/gpoptimizer/runner/ws"
)

// jobState tracks the local file paths a job's download/encode/upload
// commands hand off to each other. The server addresses all three by the
// same job_id but each arrives as a separate command.
type jobState struct {
	originalPath string
	encodedPath  string
	filename     string
}

// runnerState holds everything the command handler needs across commands:
// the Google API clients (nil until the server sends credentials), a cache
// of the video metadata from the last sync, and in-flight job paths.
type runnerState struct {
	cfg        *config.Config
	ffmpegPath string

	mu     sync.Mutex
	photos *google.PhotosClient
	drive  *google.DriveClient
	jobs   map[int]*jobState
}

func main() {
	pairCode := flag.String("pair", "", "Pairing code from the web UI")
	serverURL := flag.String("server", "https://gpoptimizer.example.com", "Server URL")
	flag.Parse()

	if *pairCode != "" {
		if _, err := pairing.Pair(*serverURL, *pairCode); err != nil {
			log.Fatalf("Pairing failed: %v", err)
		}
		fmt.Println("Paired successfully! Runner will now connect automatically.")
		fmt.Printf("Config saved to %s\n", config.Path())
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Not paired yet. Run with: --pair <CODE> --server <URL>")
		os.Exit(1)
	}

	ffmpegPath, err := ffmpeg.EnsureInstalled()
	if err != nil {
		log.Fatal(err)
	}

	s := &runnerState{
		cfg:        cfg,
		ffmpegPath: ffmpegPath,
		jobs:       make(map[int]*jobState),
	}

	ws.Run(cfg, s.handleCommand)
}

// handleCommand dispatches one command from the server to the matching
// pipeline handler. Registered as the ws.Client's command handler.
func (s *runnerState) handleCommand(c *ws.Client, cmd protocol.Command) {
	ctx := context.Background()

	switch cmd.Type {
	case "google_credentials":
		s.handleGoogleCredentials(c, cmd)
	case "download":
		s.handleDownload(ctx, c, cmd)
	case "encode":
		s.handleEncode(ctx, c, cmd)
	case "upload":
		s.handleUpload(ctx, c, cmd)
	case "delete_local":
		s.handleDeleteLocal(c, cmd)
	default:
		log.Printf("runner: unknown command type %q", cmd.Type)
	}
}

func (s *runnerState) handleGoogleCredentials(c *ws.Client, cmd protocol.Command) {
	var tok oauth2.Token
	if err := json.Unmarshal([]byte(cmd.TokenJSON), &tok); err != nil {
		log.Printf("runner: parse google token: %v", err)
		return
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cmd.ClientID,
		ClientSecret: cmd.ClientSecret,
		Scopes:       google.Scopes,
		Endpoint:     oauthgoogle.Endpoint,
	}

	drive, err := google.NewDriveClient(&tok, oauthCfg)
	if err != nil {
		log.Printf("runner: build drive client: %v", err)
		return
	}

	s.mu.Lock()
	s.photos = google.NewPhotosClient(&tok, oauthCfg)
	s.drive = drive
	s.mu.Unlock()

	log.Println("runner: Google Photos connected via server credentials")
	c.SendStatus(protocol.Status{Type: "google_auth_status", Connected: true})
}

func (s *runnerState) handleDownload(ctx context.Context, c *ws.Client, cmd protocol.Command) {
	s.mu.Lock()
	photos := s.photos
	s.mu.Unlock()

	if photos == nil {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "not connected to Google Photos; connect via Settings in the web UI"})
		return
	}
	if cmd.BaseURL == "" {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "no base_url in download command"})
		return
	}

	path, err := pipeline.HandleDownload(ctx, c.SendStatus, photos, s.cfg, cmd.VideoID, cmd.JobID, cmd.BaseURL)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.jobs[cmd.JobID] = &jobState{originalPath: path, filename: cmd.VideoID}
	s.mu.Unlock()

	// Auto-chain into encode
	s.handleEncode(ctx, c, cmd)
}

func (s *runnerState) handleEncode(ctx context.Context, c *ws.Client, cmd protocol.Command) {
	s.mu.Lock()
	job := s.jobs[cmd.JobID]
	s.mu.Unlock()

	if job == nil {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "no downloaded file for this job"})
		return
	}

	opts := ffmpeg.EncodeOpts{Codec: cmd.Codec, CRF: cmd.CRF, Preset: cmd.Preset}
	outPath, err := pipeline.HandleEncode(ctx, c.SendStatus, s.ffmpegPath, s.cfg, job.originalPath, cmd.JobID, opts, 0)
	if err != nil {
		return
	}

	s.mu.Lock()
	job.encodedPath = outPath
	s.mu.Unlock()
}

func (s *runnerState) handleUpload(ctx context.Context, c *ws.Client, cmd protocol.Command) {
	s.mu.Lock()
	job := s.jobs[cmd.JobID]
	drive := s.drive
	s.mu.Unlock()

	if drive == nil {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "not connected to Google Drive; connect via Settings in the web UI"})
		return
	}
	if job == nil || job.encodedPath == "" {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "no encoded file for this job"})
		return
	}

	origInfo, err := os.Stat(job.originalPath)
	if err != nil {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: fmt.Sprintf("upload: %v", err)})
		return
	}

	if err := pipeline.HandleUpload(ctx, c.SendStatus, drive, job.encodedPath, job.filename, origInfo.Size(), cmd.JobID, cmd.DeleteOriginal); err != nil {
		return
	}

	s.mu.Lock()
	delete(s.jobs, cmd.JobID)
	s.mu.Unlock()
}

func (s *runnerState) handleDeleteLocal(c *ws.Client, cmd protocol.Command) {
	deleted := 0
	for _, path := range cmd.Targets {
		if err := os.Remove(path); err != nil {
			log.Printf("runner: delete_local %s: %v", path, err)
			continue
		}
		deleted++
	}
	c.SendStatus(protocol.Status{Type: "delete_local_complete", Count: deleted})
}
