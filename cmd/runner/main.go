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
// the Google API clients (nil until authenticated), a cache of the video
// metadata from the last sync, and in-flight job paths.
type runnerState struct {
	cfg          *config.Config
	ffmpegPath   string
	clientID     string
	clientSecret string

	mu     sync.Mutex
	photos *google.PhotosClient
	drive  *google.DriveClient
	videos map[string]google.VideoMeta
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
		cfg:          cfg,
		ffmpegPath:   ffmpegPath,
		clientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		clientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		videos:       make(map[string]google.VideoMeta),
		jobs:         make(map[int]*jobState),
	}
	s.loadGoogleClients() // best-effort; nil clients until start_google_auth succeeds

	ws.Run(cfg, s.handleCommand)
}

// loadGoogleClients builds the Photos/Drive clients from a previously saved
// token, if any. Logs and continues (clients stay nil) if there isn't one
// yet — the user authenticates via the start_google_auth command.
func (s *runnerState) loadGoogleClients() {
	tok, err := google.LoadToken()
	if err != nil {
		log.Printf("runner: no saved Google token yet: %v", err)
		return
	}
	oauthCfg := google.NewOAuthConfig(s.clientID, s.clientSecret)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.photos = google.NewPhotosClient(tok, oauthCfg)
	drive, err := google.NewDriveClient(tok, oauthCfg)
	if err != nil {
		log.Printf("runner: build drive client: %v", err)
		return
	}
	s.drive = drive
}

// handleCommand dispatches one command from the server to the matching
// pipeline handler. Registered as the ws.Client's command handler.
func (s *runnerState) handleCommand(c *ws.Client, cmd protocol.Command) {
	ctx := context.Background()

	switch cmd.Type {
	case "download":
		s.handleDownload(ctx, c, cmd)
	case "encode":
		s.handleEncode(ctx, c, cmd)
	case "upload":
		s.handleUpload(ctx, c, cmd)
	case "sync_videos":
		s.handleSyncVideos(ctx, c)
	case "delete_local":
		s.handleDeleteLocal(c, cmd)
	case "start_google_auth":
		s.handleStartGoogleAuth(c)
	default:
		log.Printf("runner: unknown command type %q", cmd.Type)
	}
}

func (s *runnerState) handleDownload(ctx context.Context, c *ws.Client, cmd protocol.Command) {
	s.mu.Lock()
	meta, ok := s.videos[cmd.VideoID]
	photos := s.photos
	s.mu.Unlock()

	if photos == nil {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "not connected to Google Photos; run start_google_auth"})
		return
	}
	if !ok {
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "unknown video_id: run sync_videos first"})
		return
	}

	path, err := pipeline.HandleDownload(ctx, c.SendStatus, photos, s.cfg, cmd.VideoID, cmd.JobID, meta.BaseURL)
	if err != nil {
		return // pipeline already sent an error status
	}

	s.mu.Lock()
	s.jobs[cmd.JobID] = &jobState{originalPath: path, filename: meta.Filename}
	s.mu.Unlock()
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
	// ponytail: durationMs is 0 (unknown) — Google Photos' mediaItems:search
	// doesn't return video duration, so ffmpeg.Encode's progress-from-time
	// calculation is skipped; job_complete still fires at the end. Add a
	// duration probe (ffprobe, or metadata.video from Photos) if per-job
	// progress percentages during encode turn out to matter.
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
		c.SendStatus(protocol.Status{Type: "error", JobID: cmd.JobID, Message: "not connected to Google Drive; run start_google_auth"})
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

func (s *runnerState) handleSyncVideos(ctx context.Context, c *ws.Client) {
	s.mu.Lock()
	photos := s.photos
	s.mu.Unlock()
	if photos == nil {
		c.SendStatus(protocol.Status{Type: "error", Message: "not connected to Google Photos; run start_google_auth"})
		return
	}

	videos, err := photos.ListVideos(ctx)
	if err != nil {
		c.SendStatus(protocol.Status{Type: "error", Message: fmt.Sprintf("sync_videos: %v", err)})
		return
	}

	byID := make(map[string]google.VideoMeta, len(videos))
	for _, v := range videos {
		byID[v.ID] = v
	}
	s.mu.Lock()
	s.videos = byID
	s.mu.Unlock()

	data, err := json.Marshal(videos)
	if err != nil {
		c.SendStatus(protocol.Status{Type: "error", Message: fmt.Sprintf("sync_videos: %v", err)})
		return
	}
	c.SendStatus(protocol.Status{Type: "videos_synced", Count: len(videos), Videos: data})
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

func (s *runnerState) handleStartGoogleAuth(c *ws.Client) {
	tok, oauthCfg, err := google.StartLocalAuth(s.clientID, s.clientSecret)
	if err != nil {
		c.SendStatus(protocol.Status{Type: "error", Message: fmt.Sprintf("google auth: %v", err)})
		return
	}
	if err := google.SaveToken(tok); err != nil {
		log.Printf("runner: save google token: %v", err)
	}

	drive, err := google.NewDriveClient(tok, oauthCfg)
	if err != nil {
		c.SendStatus(protocol.Status{Type: "error", Message: fmt.Sprintf("google auth: %v", err)})
		return
	}

	s.mu.Lock()
	s.photos = google.NewPhotosClient(tok, oauthCfg)
	s.drive = drive
	s.mu.Unlock()

	c.SendStatus(protocol.Status{Type: "google_auth_status", Connected: true})
}
