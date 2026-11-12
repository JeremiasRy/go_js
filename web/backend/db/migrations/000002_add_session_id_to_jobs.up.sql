ALTER TABLE user_script_jobs
    ADD COLUMN IF NOT EXISTS session_id uuid;

CREATE INDEX IF NOT EXISTS user_script_jobs_session_id_created_at_idx
    ON user_script_jobs (session_id, created_at DESC);
