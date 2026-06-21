package worker

import (
	"context"
	"errors"
	"go_js_demo/db"
	"go_js_demo/microvm"
	"log"

	"github.com/jackc/pgx/v5"
)

type HeartbeatSignal struct{}

type Worker struct {
	db  *db.Db
	mvm *microvm.MicroVMHandler
	sig chan HeartbeatSignal
}

func NewWorker(db *db.Db, mvm *microvm.MicroVMHandler, sig chan HeartbeatSignal) *Worker {
	return &Worker{
		db,
		mvm,
		sig,
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

				res, err := w.mvm.RunCode(job.Src, ctx)

				var jobStatus db.JobStatus

				if err != nil {
					res = err.Error()
					jobStatus = db.JobFailed
				} else {
					jobStatus = db.JobSucceeded
				}

				err = w.db.UpdateJob(jobStatus, res, job.JobId, ctx)

				if err != nil {
					log.Printf("Worker error: %v", err)
					continue
				}
			}
		}
	}
}

func SpinUpWorkers(db *db.Db, mvm *microvm.MicroVMHandler, sig chan HeartbeatSignal, amount int, ctx context.Context) {
	for id := range amount {
		w := NewWorker(db, mvm, sig)
		go w.Run(id, ctx)
	}
}
