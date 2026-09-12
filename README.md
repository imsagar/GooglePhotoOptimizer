# GPOptimizer

Optimize your Google Photos videos via H.265 re-encoding — without losing quality, without uploading to third-party servers. Your videos never leave your machine.

**Architecture:** A web app (Go server + React frontend) manages the workflow. A local runner binary on your machine does the actual downloading, encoding, and uploading — communicating with the server over an encrypted WebSocket.

## Quick Start (Docker Compose)

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- A Google Cloud project with OAuth credentials (see [Google OAuth Setup](#google-oauth-setup))

### 1. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your values:

```env
DATABASE_URL=postgres://gpopt:gpopt@db:5432/gpoptimizer?sslmode=disable
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:5173/api/auth/google/callback
SESSION_SECRET=generate-a-random-64-char-string-here
SERVER_ADDR=:8080
```

### 2. Start the stack

```bash
docker compose up --build
```

This starts three services:

| Service | Port | Description |
|---------|------|-------------|
| **frontend** | [localhost:5173](http://localhost:5173) | React UI (nginx) |
| **server** | localhost:8080 | Go API + WebSocket hub |
| **db** | localhost:5432 | PostgreSQL 16 |

### 3. Sign in

Open [http://localhost:5173](http://localhost:5173) and sign in with Google.

---

## Runner Setup

The runner is a standalone Go binary that runs on your local machine. It handles video downloading, FFmpeg encoding, and Google Drive uploads.

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [FFmpeg](https://ffmpeg.org/download.html) installed and on your `PATH`
  - macOS: `brew install ffmpeg`
  - Linux: `apt install ffmpeg`
  - Windows: `winget install ffmpeg`

### Build the runner

```bash
go build -o gpoptimizer-runner ./cmd/runner
```

Or cross-compile for other platforms:

```bash
# macOS ARM
GOOS=darwin GOARCH=arm64 go build -o dist/gpoptimizer-runner-darwin-arm64 ./cmd/runner

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o dist/gpoptimizer-runner-darwin-amd64 ./cmd/runner

# Linux
GOOS=linux GOARCH=amd64 go build -o dist/gpoptimizer-runner-linux-amd64 ./cmd/runner

# Windows
GOOS=windows GOARCH=amd64 go build -o dist/gpoptimizer-runner-windows-amd64.exe ./cmd/runner
```

### Pair with the server

1. In the web UI, go to **Settings** and click **Generate Pairing Code**
2. Run the runner with the pairing code:

```bash
./gpoptimizer-runner --pair <CODE> --server http://localhost:8080
```

This saves the connection config to `~/.gpoptimizer/config.json`.

### Run the runner

```bash
export GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
export GOOGLE_CLIENT_SECRET=your-client-secret
./gpoptimizer-runner
```

On first run, the runner will open your browser for Google OAuth to authorize access to Google Photos and Google Drive. The token is saved locally at `~/.gpoptimizer/google_token.json`.

Once connected, the runner appears as "Online" in the web UI dashboard.

---

## Google OAuth Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/) > **APIs & Services** > **Credentials**
2. Create an **OAuth 2.0 Client ID** (Application type: Web application)
3. Add authorized redirect URI: `http://localhost:5173/api/auth/google/callback`
4. Enable these APIs in your project:
   - **Google Photos Library API** — runner lists and downloads videos
   - **Google Drive API** — runner uploads optimized videos
5. Copy the Client ID and Client Secret into your `.env` file

The same Client ID / Secret is used by both the server (for user login) and the runner (for Photos/Drive access).

---

## Development

### Run server locally (without Docker)

```bash
# Start Postgres (Docker or local install)
docker compose up db -d

# Set env vars
export DATABASE_URL=postgres://gpopt:gpopt@localhost:5432/gpoptimizer?sslmode=disable
export GOOGLE_CLIENT_ID=...
export GOOGLE_CLIENT_SECRET=...
export GOOGLE_REDIRECT_URL=http://localhost:5173/api/auth/google/callback
export SESSION_SECRET=dev-secret-change-in-prod

# Run server
go run ./cmd/server
```

### Run frontend locally

```bash
cd frontend
npm install
npm run dev
```

The Vite dev server runs on port 5173 and proxies `/api` and `/ws` to `localhost:8080`.

### Run tests

```bash
# Go tests
go test ./...

# Frontend type check
cd frontend && npx tsc --noEmit
```

---

## How It Works

```
Browser (React)  <-->  Server (Go/Gin)  <--(encrypted WS)-->  Runner (Go binary)
                           |                                       |
                       PostgreSQL                            FFmpeg + Google APIs
```

1. **Sign in** with Google via the web UI
2. **Pair** your local runner with a one-time code
3. **Sync** your Google Photos video library
4. **Select** videos to optimize (sorted by size, filtered by date/album)
5. **Runner downloads** the video from Google Photos
6. **Runner encodes** with FFmpeg (H.265, configurable CRF/preset)
7. **Runner uploads** the optimized video to Google Drive
8. **Verify** upload integrity (size check)
9. Optionally **delete the original** from Google Drive

All communication between server and runner is encrypted with AES-256-GCM (HKDF-SHA256 derived from the runner token).

---

## Project Structure

```
gpoptimizer/
├── cmd/
│   ├── server/          # Web server entry point
│   └── runner/          # Runner binary entry point
├── server/
│   ├── auth/            # Google OAuth + session middleware
│   ├── handler/         # REST + WebSocket handlers
│   ├── relay/           # Message relay (server ↔ runner)
│   └── store/           # Postgres data access
├── runner/
│   ├── config/          # Runner config (~/.gpoptimizer/)
│   ├── ffmpeg/          # FFmpeg encode wrapper
│   ├── google/          # Photos + Drive API clients
│   ├── pipeline/        # Download → Encode → Upload pipeline
│   └── ws/              # WebSocket client with reconnect
├── internal/
│   ├── crypto/          # AES-256-GCM encryption
│   └── protocol/        # Shared message types
├── frontend/            # React + TypeScript + Vite + Tailwind
│   └── src/
│       ├── api/         # Typed API client
│       ├── components/  # Layout, Sidebar, StatusBadge, ProgressBar
│       ├── hooks/       # useWebSocket
│       └── pages/       # Login, Dashboard, Videos, Jobs, LocalFiles, Settings
├── migrations/          # PostgreSQL schema
├── docker-compose.yml
├── Dockerfile.server
└── .env.example
```

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Server | Go, Gin, gorilla/websocket, PostgreSQL 16 |
| Runner | Go, FFmpeg (subprocess), Google Photos/Drive APIs |
| Frontend | React 19, TypeScript, Vite, Tailwind CSS 4 |
| Encryption | AES-256-GCM, HKDF-SHA256 |
| Dev Environment | Docker Compose |

---

## Environment Variables Reference

### Server

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | Postgres connection string |
| `GOOGLE_CLIENT_ID` | Yes | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | Yes | OAuth callback URL |
| `SESSION_SECRET` | Yes | Cookie signing secret (64+ random chars) |
| `SERVER_ADDR` | No | Listen address (default: `:8080`) |

### Runner

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_CLIENT_ID` | Yes | Same Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | Same Google OAuth client secret |

Runner config (created by `--pair`) is stored at `~/.gpoptimizer/config.json`.
