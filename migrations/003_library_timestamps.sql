-- migrations/003_library_timestamps.sql
-- Adds timestamp columns for the Library page: when each phase completed.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS downloaded_at TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS optimized_at TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS uploaded_at TIMESTAMPTZ;
