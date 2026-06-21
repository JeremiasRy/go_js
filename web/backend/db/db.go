package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobStatus int

const (
	JobPending JobStatus = iota
	JobProcessing
	JobFailed
	JobSucceeded
)

func (j *JobStatus) String() string {
	if *j == JobPending {
		return "Pending"
	}
	if *j == JobProcessing {
		return "Processing"
	}
	if *j == JobFailed {
		return "Failed"
	}
	if *j == JobSucceeded {
		return "Success"
	}
	panic("no-op")
}

type UserScriptJob struct {
	JobId     uuid.UUID `json:"job_id"`
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
	log.Printf("DB=%s", os.Getenv("DATABASE_URL"))
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))

	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	return &Db{pool}
}

func (db *Db) InsertNewJob(src string, ctx context.Context) (string, error) {
	query := `
        INSERT INTO user_script_jobs (src, job_status) 
        VALUES ($1, $2) 
        RETURNING *`

	var job UserScriptJob
	err := db.pool.QueryRow(ctx, query, src, JobPending).Scan(
		&job.JobId,
		&job.Src,
		&job.JobStatus,
		&job.Result,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return "", fmt.Errorf("failed to insert job: %w", err)
	}

	return job.JobId.String(), nil
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
    RETURNING *`

	var job UserScriptJob
	err := db.pool.QueryRow(ctx, query, JobProcessing, JobPending).Scan(
		&job.JobId,
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
	SELECT * from user_script_jobs
	WHERE job_id = $1
	`
	var job UserScriptJob
	err := db.pool.QueryRow(ctx, query, jobId).Scan(
		&job.JobId,
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
