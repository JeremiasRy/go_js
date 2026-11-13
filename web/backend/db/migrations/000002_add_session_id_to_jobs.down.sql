DROP INDEX IF EXISTS user_script_jobs_session_id_created_at_idx;

ALTER TABLE user_script_jobs
    DROP COLUMN IF EXISTS session_id;
