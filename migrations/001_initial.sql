-- migrations/001_initial.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT UNIQUE NOT NULL,
    name       TEXT,
    avatar_url TEXT,
    google_token TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE runners (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash       TEXT NOT NULL,
    label            TEXT,
    platform         TEXT,
    arch             TEXT,
    ffmpeg_version   TEXT,
    google_connected BOOLEAN DEFAULT FALSE,
    last_seen_at     TIMESTAMPTZ,
    status           TEXT DEFAULT 'offline',
    created_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE videos (
    id              TEXT NOT NULL,
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    google_drive_id TEXT,
    filename        TEXT NOT NULL,
    mime_type       TEXT,
    size_bytes      BIGINT,
    duration_ms     INTEGER,
    width           INTEGER,
    height          INTEGER,
    creation_time   TIMESTAMPTZ,
    album_id        TEXT,
    album_title     TEXT,
    synced_at       TIMESTAMPTZ,
    PRIMARY KEY (id, user_id)
);

CREATE INDEX idx_videos_user_size ON videos(user_id, size_bytes DESC);
CREATE INDEX idx_videos_user_date ON videos(user_id, creation_time DESC);

CREATE TABLE jobs (
    id                SERIAL PRIMARY KEY,
    user_id           UUID REFERENCES users(id) ON DELETE CASCADE,
    video_id          TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'queued',
    run_date          TEXT NOT NULL,
    original_size     BIGINT,
    optimized_size    BIGINT,
    savings_pct       REAL,
    codec             TEXT DEFAULT 'libx265',
    crf               INTEGER DEFAULT 18,
    preset            TEXT DEFAULT 'medium',
    error             TEXT,
    progress          INTEGER DEFAULT 0,
    delete_original   BOOLEAN DEFAULT FALSE,
    size_verified     BOOLEAN,
    duration_verified BOOLEAN,
    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_jobs_user_status ON jobs(user_id, status);

CREATE TABLE pairing_codes (
    code       TEXT PRIMARY KEY,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
