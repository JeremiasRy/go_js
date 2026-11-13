package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"go_js_demo/db"
)

const scriptQueryTimeout = 5 * time.Second

type scriptSummary struct {
	JobID     string       `json:"job_id"`
	SessionID string       `json:"session_id"`
	Src       string       `json:"src"`
	JobStatus db.JobStatus `json:"job_status"`
	CreatedAt time.Time    `json:"created_at"`
}

func (m *Manager) HandleSessionJobs(w http.ResponseWriter, r *http.Request) {
	sessionID, err := canonicalSessionID(r.PathValue("session_id"))
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	if m.store == nil {
		http.Error(w, "script storage is unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), scriptQueryTimeout)
	defer cancel()

	jobs, err := m.store.GetJobsBySessionID(sessionID, ctx)
	if err != nil {
		log.Printf("Session %s could not list scripts: %v", shortID(sessionID), err)
		http.Error(w, "failed to load scripts", http.StatusInternalServerError)
		return
	}

	summaries := make([]scriptSummary, 0, len(jobs))
	for _, job := range jobs {
		summaries = append(summaries, summarizeJob(job))
	}

	writeJSON(w, summaries)
}

func (m *Manager) HandleRandomJob(w http.ResponseWriter, r *http.Request) {
	if m.store == nil {
		http.Error(w, "script storage is unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), scriptQueryTimeout)
	defer cancel()

	job, err := m.store.GetRandomJob(ctx)
	if errors.Is(err, db.ErrNoJobs) {
		http.Error(w, "no scripts available", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Could not load a random script: %v", err)
		http.Error(w, "failed to load a random script", http.StatusInternalServerError)
		return
	}

	writeJSON(w, summarizeJob(*job))
}

func summarizeJob(job db.UserScriptJob) scriptSummary {
	return scriptSummary{
		JobID:     job.JobId.String(),
		SessionID: job.SessionID,
		Src:       job.Src,
		JobStatus: job.JobStatus,
		CreatedAt: job.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("Could not write JSON response: %v", err)
	}
}
