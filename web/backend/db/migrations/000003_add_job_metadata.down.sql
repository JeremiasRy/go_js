DROP INDEX IF EXISTS user_script_jobs_status_created_at_idx;
DROP INDEX IF EXISTS user_script_jobs_session_client_request_idx;

ALTER TABLE user_script_jobs
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS revision,
    DROP COLUMN IF EXISTS output,
    DROP COLUMN IF EXISTS result_format_version,
    DROP COLUMN IF EXISTS result_encoding,
    DROP COLUMN IF EXISTS client_request_id;
