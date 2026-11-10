package worker

import (
	"context"
	"errors"
	"fmt"
	"go_js_demo/db"
	"go_js_demo/microvm"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type HeartbeatSignal struct{}

type JobResult struct {
	JobID  uuid.UUID
	Status string
	Result string
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
	for range w.sig {
		select {
		case <-ctx.Done():
			return
		default:
			{
				log.Printf("Worker %d, querying for work...", id)
				job, err := w.db.GetOldestPendingJob(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					log.Printf("No jobs")
					continue
				}

				if err != nil {
					log.Printf("Worker error: %v", err)
					continue
				}

				res, runErr := w.mvm.RunCode(job.Src, ctx)

				jobStatus := db.JobSucceeded
				if runErr != nil {
					res = runErr.Error()
					jobStatus = db.JobFailed
				}

				updateErr := w.db.UpdateJob(jobStatus, res, job.JobId, ctx)
				if updateErr != nil {
					log.Printf("Worker error: %v", updateErr)
					jobStatus = db.JobFailed
					res = fmt.Sprintf("failed to persist job result: %v", updateErr)
				}

				select {
				case w.results <- JobResult{
					JobID:  job.JobId,
					Status: jobStatus.String(),
					Result: res,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func SpinUpWorkers(db *db.Db, mvm *microvm.MicroVMHandler, sig chan HeartbeatSignal, results chan<- JobResult, amount int, ctx context.Context) {
	for id := range amount {
		w := NewWorker(db, mvm, sig, results)
		go w.Run(id, ctx)
	}
}
