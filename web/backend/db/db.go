package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
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

const jobMetadataColumns = `
    job_id,
    COALESCE(session_id::text, '') AS session_id,
    src,
    job_status,
    COALESCE(client_request_id, '') AS client_request_id,
    COALESCE(result_encoding, 'base64') AS result_encoding,
    COALESCE(result_format_version, 0) AS result_format_version,
    COALESCE(output, '') AS output,
    revision,
    created_at,
    updated_at,
    completed_at`

const jobResultColumns = jobMetadataColumns + `, result`

var (
	ErrNoJobs                = errors.New("no script jobs found")
	ErrJobNotFound           = errors.New("job not found")
	ErrClientRequestConflict = errors.New("client_request_id already used with different source")
)

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
	JobID               uuid.UUID  `json:"job_id"`
	SessionID           string     `json:"session_id"`
	Src                 string     `json:"src"`
	JobStatus           JobStatus  `json:"job_status"`
	Result              string     `json:"-"`
	ResultEncoding      string     `json:"-"`
	ResultFormatVersion int        `json:"-"`
	Output              string     `json:"-"`
	ClientRequestID     string     `json:"client_request_id,omitempty"`
	Revision            int64      `json:"revision"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

type Completion struct {
	Status              JobStatus
	Result              string
	ResultEncoding      string
	ResultFormatVersion int
	Output              string
}

type Db struct {
	pool *pgxpool.Pool
}

type jobScanner interface {
	Scan(dest ...any) error
}

func NewDatabase() *Db {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))

	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	return &Db{pool}
}

func scanJobMetadata(row jobScanner) (*UserScriptJob, error) {
	var job UserScriptJob
	err := row.Scan(
		&job.JobID,
		&job.SessionID,
		&job.Src,
		&job.JobStatus,
		&job.ClientRequestID,
		&job.ResultEncoding,
		&job.ResultFormatVersion,
		&job.Output,
		&job.Revision,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func scanJobResult(row jobScanner) (*UserScriptJob, error) {
	var job UserScriptJob
	err := row.Scan(
		&job.JobID,
		&job.SessionID,
		&job.Src,
		&job.JobStatus,
		&job.ClientRequestID,
		&job.ResultEncoding,
		&job.ResultFormatVersion,
		&job.Output,
		&job.Revision,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.CompletedAt,
		&job.Result,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func parseSessionID(sessionID string) (uuid.UUID, error) {
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session id: %w", err)
	}
	return sessionUUID, nil
}

func (db *Db) InsertNewJob(sessionID, clientRequestID, src string, ctx context.Context) (*UserScriptJob, bool, error) {
	sessionUUID, err := parseSessionID(sessionID)
	if err != nil {
		return nil, false, err
	}

	requestID := strings.TrimSpace(clientRequestID)
	var requestValue any
	if requestID != "" {
		requestValue = requestID
	}

	query := `
        INSERT INTO user_script_jobs (session_id, src, job_status, client_request_id)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (session_id, client_request_id) DO NOTHING
        RETURNING ` + jobMetadataColumns

	job, err := scanJobMetadata(db.pool.QueryRow(ctx, query, sessionUUID, src, JobPending, requestValue))
	if err == nil {
		return job, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("failed to insert job: %w", err)
	}
	if requestID == "" {
		return nil, false, fmt.Errorf("failed to insert job: %w", err)
	}

	existing, err := db.GetJobBySessionAndClientRequestID(sessionID, requestID, ctx)
	if err != nil {
		return nil, false, err
	}
	if existing.Src != src {
		return nil, false, ErrClientRequestConflict
	}
	return existing, false, nil
}

func (db *Db) MarkOldestPendingJobProcessing(ctx context.Context) (*UserScriptJob, error) {
	query := `
    UPDATE user_script_jobs
    SET job_status = $1, updated_at = NOW(), revision = revision + 1
    WHERE job_id = (
        SELECT job_id
        FROM user_script_jobs
        WHERE job_status = $2
        ORDER BY created_at ASC
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    )
    RETURNING ` + jobMetadataColumns

	job, err := scanJobMetadata(db.pool.QueryRow(ctx, query, JobProcessing, JobPending))
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (db *Db) UpdateJob(jobID uuid.UUID, completion Completion, ctx context.Context) (*UserScriptJob, error) {
	query := `
    UPDATE user_script_jobs
    SET job_status = $1,
        result = $2,
        result_encoding = $3,
        result_format_version = $4,
        output = $5,
        revision = revision + 1,
        updated_at = NOW(),
        completed_at = NOW()
    WHERE job_id = $6
    RETURNING ` + jobMetadataColumns

	job, err := scanJobMetadata(db.pool.QueryRow(
		ctx,
		query,
		completion.Status,
		completion.Result,
		completion.ResultEncoding,
		completion.ResultFormatVersion,
		completion.Output,
		jobID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}
	return job, nil
}

func (db *Db) FailStaleProcessing(before time.Time, reason string, ctx context.Context) (int64, error) {
	query := `
    UPDATE user_script_jobs
    SET job_status = $1,
        result = $2,
        result_encoding = 'text',
        result_format_version = 0,
        output = $2,
        revision = revision + 1,
        updated_at = NOW(),
        completed_at = NOW()
    WHERE job_status = $3 AND updated_at < $4`

	cmdTag, err := db.pool.Exec(ctx, query, JobFailed, reason, JobProcessing, before)
	if err != nil {
		return 0, fmt.Errorf("failed to fail stale jobs: %w", err)
	}
	return cmdTag.RowsAffected(), nil
}

func (db *Db) GetJobByJobId(jobId uuid.UUID, ctx context.Context) (*UserScriptJob, error) {
	query := `
	SELECT ` + jobMetadataColumns + `
	FROM user_script_jobs
	WHERE job_id = $1
	`
	job, err := scanJobMetadata(db.pool.QueryRow(ctx, query, jobId))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (db *Db) GetJobBySessionAndID(sessionID string, jobID uuid.UUID, ctx context.Context) (*UserScriptJob, error) {
	sessionUUID, err := parseSessionID(sessionID)
	if err != nil {
		return nil, err
	}

	query := `
	SELECT ` + jobMetadataColumns + `
	FROM user_script_jobs
	WHERE job_id = $1 AND session_id = $2
	`
	job, err := scanJobMetadata(db.pool.QueryRow(ctx, query, jobID, sessionUUID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (db *Db) GetJobResultBySessionAndID(sessionID string, jobID uuid.UUID, ctx context.Context) (*UserScriptJob, error) {
	sessionUUID, err := parseSessionID(sessionID)
	if err != nil {
		return nil, err
	}

	query := `
	SELECT ` + jobResultColumns + `
	FROM user_script_jobs
	WHERE job_id = $1 AND session_id = $2
	`
	job, err := scanJobResult(db.pool.QueryRow(ctx, query, jobID, sessionUUID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (db *Db) GetJobBySessionAndClientRequestID(sessionID, clientRequestID string, ctx context.Context) (*UserScriptJob, error) {
	sessionUUID, err := parseSessionID(sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(clientRequestID) == "" {
		return nil, ErrJobNotFound
	}

	query := `
	SELECT ` + jobMetadataColumns + `
	FROM user_script_jobs
	WHERE session_id = $1 AND client_request_id = $2
	`
	job, err := scanJobMetadata(db.pool.QueryRow(ctx, query, sessionUUID, clientRequestID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (db *Db) GetJobsBySessionID(sessionID string, ctx context.Context) ([]UserScriptJob, error) {
	sessionUUID, err := parseSessionID(sessionID)
	if err != nil {
		return nil, err
	}

	query := `
	SELECT ` + jobMetadataColumns + `
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
		job, err := scanJobMetadata(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read jobs: %w", err)
	}

	return jobs, nil
}

func (db *Db) GetRandomJob(ctx context.Context) (*UserScriptJob, error) {
	query := `
	SELECT ` + jobMetadataColumns + `
	FROM user_script_jobs
	WHERE job_status <> $1
	ORDER BY random()
	LIMIT 1
	`

	job, err := scanJobMetadata(db.pool.QueryRow(ctx, query, JobFailed))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoJobs
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get random job: %w", err)
	}
	return job, nil
}
