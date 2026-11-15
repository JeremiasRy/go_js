package worker

import (
	"context"
	"errors"
	"fmt"
	"go_js_demo/db"
	"go_js_demo/jobresult"
	"go_js_demo/microvm"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const pollInterval = 2 * time.Second

type HeartbeatSignal struct{}

type JobResult struct {
	SessionID       string
	ClientRequestID string
	JobID           uuid.UUID
	Status          db.JobStatus
	Revision        int64
	Output          string
	ErrorMessage    string
}

type Worker struct {
	db      *db.Db
	mvm     *microvm.MicroVMHandler
	sig     chan HeartbeatSignal
	results chan<- JobResult
}

func NewWorker(db *db.Db, mvm *microvm.MicroVMHandler, sig chan HeartbeatSignal, results chan<- JobResult) *Worker {
	return &Worker{
		db,
		mvm,
		sig,
		results,
	}
}

func (w *Worker) Run(id int, ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	w.processOne(id, ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-w.sig:
			if !ok {
				return
			}
			w.processOne(id, ctx)
		case <-ticker.C:
			w.processOne(id, ctx)
		}
	}
}

func (w *Worker) processOne(id int, ctx context.Context) {
	job, err := w.db.MarkOldestPendingJobProcessing(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		log.Printf("Worker %d could not claim a job: %v", id, err)
		return
	}

	processing := JobResult{
		SessionID:       job.SessionID,
		ClientRequestID: job.ClientRequestID,
		JobID:           job.JobID,
		Status:          db.JobProcessing,
		Revision:        job.Revision,
	}
	select {
	case w.results <- processing:
	case <-ctx.Done():
		return
	default:
		log.Printf("Worker %d could not publish processing status for job %s", id, shortJobID(job.JobID))
	}

	res, runErr := w.mvm.RunCode(job.Src, ctx)
	completion := completionFor(res, runErr)

	updated, updateErr := w.db.UpdateJob(job.JobID, completion, ctx)
	if updateErr != nil {
		log.Printf("Worker %d failed to persist job %s: %v", id, shortJobID(job.JobID), updateErr)

		fallback := db.Completion{
			Status:         db.JobFailed,
			Result:         fmt.Sprintf("failed to persist job result: %v", updateErr),
			ResultEncoding: "text",
		}
		fallback.Output = fallback.Result
		fallbackJob, fallbackErr := w.db.UpdateJob(job.JobID, fallback, ctx)
		if fallbackErr != nil {
			log.Printf("Worker %d failed to record fallback failure for job %s: %v", id, shortJobID(job.JobID), fallbackErr)
			w.publishTerminal(ctx, JobResult{
				SessionID:       job.SessionID,
				ClientRequestID: job.ClientRequestID,
				JobID:           job.JobID,
				Status:          db.JobFailed,
				Revision:        job.Revision,
				ErrorMessage:    fallback.Result,
			})
			return
		}
		updated = fallbackJob
		completion = fallback
	}

	terminal := JobResult{
		SessionID:       updated.SessionID,
		ClientRequestID: updated.ClientRequestID,
		JobID:           updated.JobID,
		Status:          updated.JobStatus,
		Revision:        updated.Revision,
	}
	if updated.JobStatus == db.JobSucceeded {
		terminal.Output = completion.Output
	} else {
		terminal.ErrorMessage = completion.Result
	}

	w.publishTerminal(ctx, terminal)
}

func completionFor(res string, runErr error) db.Completion {
	if runErr != nil {
		message := runErr.Error()
		return db.Completion{
			Status:         db.JobFailed,
			Result:         message,
			ResultEncoding: "text",
			Output:         message,
		}
	}

	decoded, decodeErr := jobresult.Decode(res)
	if decodeErr != nil {
		message := "invalid structured result: " + decodeErr.Error()
		return db.Completion{
			Status:         db.JobFailed,
			Result:         message,
			ResultEncoding: "text",
			Output:         message,
		}
	}

	return db.Completion{
		Status:              db.JobSucceeded,
		Result:              res,
		ResultEncoding:      "base64",
		ResultFormatVersion: decoded.FormatVersion,
		Output:              decoded.Output,
	}
}

func (w *Worker) publishTerminal(ctx context.Context, terminal JobResult) {
	select {
	case w.results <- terminal:
	case <-ctx.Done():
	}
}

func shortJobID(id uuid.UUID) string {
	value := id.String()
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

func SpinUpWorkers(db *db.Db, mvm *microvm.MicroVMHandler, sig chan HeartbeatSignal, results chan<- JobResult, amount int, ctx context.Context) {
	for id := range amount {
		w := NewWorker(db, mvm, sig, results)
		go w.Run(id, ctx)
	}
}
