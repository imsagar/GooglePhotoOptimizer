-- migrations/002_picker_api.sql
-- Adds base_url to videos for Picker API download URLs,
-- and picker_session_id to users for re-fetching fresh URLs.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS base_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS picker_session_id TEXT;
