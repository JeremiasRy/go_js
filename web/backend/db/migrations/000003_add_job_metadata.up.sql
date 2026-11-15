ALTER TABLE user_script_jobs
    ADD COLUMN IF NOT EXISTS client_request_id text,
    ADD COLUMN IF NOT EXISTS result_encoding text NOT NULL DEFAULT 'base64',
    ADD COLUMN IF NOT EXISTS result_format_version integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS completed_at timestamp WITH TIME ZONE;

CREATE UNIQUE INDEX IF NOT EXISTS user_script_jobs_session_client_request_idx
    ON user_script_jobs (session_id, client_request_id);

CREATE INDEX IF NOT EXISTS user_script_jobs_status_created_at_idx
    ON user_script_jobs (job_status, created_at);
