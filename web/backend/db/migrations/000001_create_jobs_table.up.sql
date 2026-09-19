CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE if not exists user_script_jobs (
    job_id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    src        text NOT NULL,
    job_status int NOT NULL,
    result     text NOT NULL default '',
    created_at timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);