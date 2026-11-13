package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobStatus int

const (
	JobPending JobStatus = iota
	JobProcessing
	JobFailed
	JobSucceeded
)

const jobColumns = `job_id, COALESCE(session_id::text, '') AS session_id, src, job_status, result, created_at, updated_at`

var ErrNoJobs = errors.New("no script jobs found")

func (j JobStatus) String() string {
	if j == JobPending {
		return "Pending"
	}
	if j == JobProcessing {
		return "Processing"
	}
	if j == JobFailed {
		return "Failed"
	}
	if j == JobSucceeded {
		return "Success"
	}
	panic("no-op")
}

func (j JobStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j *JobStatus) UnmarshalJSON(data []byte) error {
	var status string
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	switch status {
	case "Pending":
		*j = JobPending
	case "Processing":
		*j = JobProcessing
	case "Failed":
		*j = JobFailed
	case "Success":
		*j = JobSucceeded
	default:
		return fmt.Errorf("unknown job status: %s", status)
	}
	return nil
}

type UserScriptJob struct {
	JobId     uuid.UUID `json:"job_id"`
	SessionID string    `json:"session_id"`
	Src       string    `json:"src"`
	JobStatus JobStatus `json:"job_status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Db struct {
	pool *pgxpool.Pool
}

func NewDatabase() *Db {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))

	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	return &Db{pool}
}

func (db *Db) InsertNewJob(sessionID string, src string, ctx context.Context) (uuid.UUID, error) {
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session id: %w", err)
	}

	query := `
        INSERT INTO user_script_jobs (session_id, src, job_status)
        VALUES ($1, $2, $3)
        RETURNING ` + jobColumns

	var job UserScriptJob
	err = db.pool.QueryRow(ctx, query, sessionUUID, src, JobPending).Scan(
		&job.JobId,
		&job.SessionID,
		&job.Src,
		&job.JobStatus,
		&job.Result,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert job: %w", err)
	}

	return job.JobId, nil
}

func (db *Db) GetOldestPendingJob(ctx context.Context) (*UserScriptJob, error) {
	query := `
    UPDATE user_script_jobs
    SET job_status = $1, updated_at = NOW()
    WHERE job_id = (
        SELECT job_id
        FROM user_script_jobs
        WHERE job_status = $2
        ORDER BY created_at ASC
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    )
    RETURNING ` + jobColumns

	var job UserScriptJob
	err := db.pool.QueryRow(ctx, query, JobProcessing, JobPending).Scan(
		&job.JobId,
		&job.SessionID,
		&job.Src,
		&job.JobStatus,
		&job.Result,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (db *Db) UpdateJob(status JobStatus, result string, jobId uuid.UUID, ctx context.Context) error {
	query := `
    UPDATE user_script_jobs
    SET job_status = $1, updated_at = NOW(), result = $2
    WHERE job_id = $3`

	cmdTag, err := db.pool.Exec(ctx, query, status, result, jobId)

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no job found with ID: %s", jobId)
	}

	return nil
}

func (db *Db) GetJobByJobId(jobId uuid.UUID, ctx context.Context) (*UserScriptJob, error) {
	query := `
	SELECT ` + jobColumns + `
	FROM user_script_jobs
	WHERE job_id = $1
	`
	var job UserScriptJob
	err := db.pool.QueryRow(ctx, query, jobId).Scan(
		&job.JobId,
		&job.SessionID,
		&job.Src,
		&job.JobStatus,
		&job.Result,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (db *Db) GetJobsBySessionID(sessionID string, ctx context.Context) ([]UserScriptJob, error) {
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session id: %w", err)
	}

	query := `
	SELECT ` + jobColumns + `
	FROM user_script_jobs
	WHERE session_id = $1
	ORDER BY created_at DESC
	LIMIT 100
	`

	rows, err := db.pool.Query(ctx, query, sessionUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]UserScriptJob, 0)
	for rows.Next() {
		var job UserScriptJob
		if err := rows.Scan(
			&job.JobId,
			&job.SessionID,
			&job.Src,
			&job.JobStatus,
			&job.Result,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read jobs: %w", err)
	}

	return jobs, nil
}

func (db *Db) GetRandomJob(ctx context.Context) (*UserScriptJob, error) {
	query := `
	SELECT ` + jobColumns + `
	FROM user_script_jobs
	WHERE job_status <> $1
	ORDER BY random()
	LIMIT 1
	`

	var job UserScriptJob
	err := db.pool.QueryRow(ctx, query, JobFailed).Scan(
		&job.JobId,
		&job.SessionID,
		&job.Src,
		&job.JobStatus,
		&job.Result,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoJobs
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get random job: %w", err)
	}

	return &job, nil
}
