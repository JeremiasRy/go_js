package mcpserver

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"go_js_demo/db"
	"go_js_demo/jobresult"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxJobBodySize = maxCodeSize + 16*1024

const largeResultCompressionThreshold = 10 << 20

type createJobRequest struct {
	ClientRequestID  string `json:"client_request_id"`
	ExecuteRequestID string `json:"execute_request_id"`
	Code             string `json:"code"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type createJobResponse struct {
	JobID           string    `json:"job_id"`
	RequestID       string    `json:"request_id"`
	ClientRequestID string    `json:"client_request_id"`
	JobStatus       string    `json:"job_status"`
	Version         int64     `json:"version"`
	Created         bool      `json:"created"`
	StatusURL       string    `json:"status_url"`
	ResultURL       *string   `json:"result_url"`
	Error           *apiError `json:"error,omitempty"`
}

type jobStatusResponse struct {
	JobID           string     `json:"job_id"`
	RequestID       string     `json:"request_id,omitempty"`
	ClientRequestID string     `json:"client_request_id,omitempty"`
	JobStatus       string     `json:"job_status"`
	Version         int64      `json:"version"`
	CreatedAt       time.Time  `json:"created_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	StatusURL       string     `json:"status_url,omitempty"`
	ResultURL       string     `json:"result_url,omitempty"`
	Error           *apiError  `json:"error,omitempty"`
}

func (m *Manager) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	sessionID, err := canonicalSessionID(r.PathValue("session_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if m.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "storage_unavailable", "script storage is unavailable")
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxJobBodySize)
	defer body.Close()

	var request createJobRequest
	if err := json.NewDecoder(body).Decode(&request); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "code_too_large", "the submitted code is too large")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid job request")
		return
	}

	request.ClientRequestID = strings.TrimSpace(request.ClientRequestID)
	request.ExecuteRequestID = strings.TrimSpace(request.ExecuteRequestID)
	if request.ClientRequestID == "" || len(request.ClientRequestID) > maxClientRequestID {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", ErrInvalidClientRequestID.Error())
		return
	}
	if len(request.Code) > maxCodeSize {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "code_too_large", ErrCodeTooLarge.Error())
		return
	}
	if request.ExecuteRequestID != "" && request.ExecuteRequestID != request.ClientRequestID {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "execute_request_id must match client_request_id")
		return
	}

	session := m.getOrCreateSession(sessionID)
	session.touch()

	result, err := session.Submit(r.Context(), SubmitRequest{
		ClientRequestID:  request.ClientRequestID,
		ExecuteRequestID: request.ExecuteRequestID,
		Code:             request.Code,
	})
	if err != nil {
		writeSubmitError(w, err)
		return
	}

	response := createJobResponse{
		JobID:           result.Job.JobID.String(),
		RequestID:       result.RequestID,
		ClientRequestID: result.Job.ClientRequestID,
		JobStatus:       result.Job.JobStatus.String(),
		Version:         result.Job.Revision,
		Created:         result.Created,
		StatusURL:       jobStatusPath(sessionID, result.Job.JobID),
	}
	if result.Job.JobStatus == db.JobSucceeded {
		resultURL := jobResultPath(sessionID, result.Job.JobID)
		response.ResultURL = &resultURL
	}
	if result.Job.JobStatus == db.JobFailed {
		response.Error = &apiError{Code: "execution_failed", Message: result.Job.Output}
	}

	w.Header().Set("Location", response.StatusURL)
	w.Header().Set("Cache-Control", "no-store")
	writeJSONStatus(w, http.StatusAccepted, response)
}

func (m *Manager) HandleJobStatus(w http.ResponseWriter, r *http.Request) {
	sessionID, err := canonicalSessionID(r.PathValue("session_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	jobID, err := uuid.Parse(strings.TrimSpace(r.PathValue("job_id")))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if m.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "storage_unavailable", "script storage is unavailable")
		return
	}

	session := m.getOrCreateSession(sessionID)
	session.touch()

	job, err := m.store.GetJobBySessionAndID(sessionID, jobID, r.Context())
	if errors.Is(err, db.ErrJobNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("Could not load job %s for session %s: %v", shortID(jobID.String()), shortID(sessionID), err)
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load job status")
		return
	}

	writeJSONStatus(w, http.StatusOK, jobStatusResponseFor(sessionID, *job))
}

func (m *Manager) HandleJobResult(w http.ResponseWriter, r *http.Request) {
	sessionID, err := canonicalSessionID(r.PathValue("session_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	jobID, err := uuid.Parse(strings.TrimSpace(r.PathValue("job_id")))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if m.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "storage_unavailable", "script storage is unavailable")
		return
	}

	session := m.getOrCreateSession(sessionID)
	session.touch()

	job, err := m.store.GetJobResultBySessionAndID(sessionID, jobID, r.Context())
	if errors.Is(err, db.ErrJobNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("Could not load result for job %s in session %s: %v", shortID(jobID.String()), shortID(sessionID), err)
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load job result")
		return
	}

	switch job.JobStatus {
	case db.JobPending, db.JobProcessing:
		w.Header().Set("Location", jobStatusPath(sessionID, job.JobID))
		w.Header().Set("Retry-After", "1")
		writeJSONStatus(w, http.StatusAccepted, jobStatusResponseFor(sessionID, *job))
		return
	case db.JobFailed:
		writeAPIError(w, http.StatusConflict, "execution_failed", job.Result)
		return
	case db.JobSucceeded:
	}

	var document []byte
	switch job.ResultEncoding {
	case "json":
		document = []byte(job.Result)
	case "", "base64":
		document, err = jobresult.LegacyJSON(job.Result)
		if err != nil {
			log.Printf("Stored result for job %s is not valid base64: %v", shortID(jobID.String()), err)
			writeAPIError(w, http.StatusInternalServerError, "invalid_stored_result", "the stored result is corrupt")
			return
		}
	default:
		writeAPIError(w, http.StatusInternalServerError, "invalid_stored_result", "the stored result encoding is not supported")
		return
	}

	if !json.Valid(document) {
		log.Printf("Stored result for job %s is not valid JSON", shortID(jobID.String()))
		writeAPIError(w, http.StatusInternalServerError, "invalid_stored_result", "the stored result is corrupt")
		return
	}

	acceptEncoding := r.Header.Get("Accept-Encoding")
	gzipAccepted, gzipRefused := gzipPreference(acceptEncoding)
	gzipEncoding := gzipAccepted || (len(document) >= largeResultCompressionThreshold && !gzipRefused)
	etag := `"` + jobID.String() + "-" + strconv.FormatInt(job.Revision, 10) + `-`
	if gzipEncoding {
		etag += "gzip"
	} else {
		etag += "identity"
	}
	etag += `"`

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("ETag", etag)

	if ifNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if gzipEncoding {
		w.Header().Set("Content-Encoding", "gzip")
		writer := gzip.NewWriter(w)
		if _, err := writer.Write(document); err != nil {
			log.Printf("Could not write gzip result for job %s: %v", shortID(jobID.String()), err)
			_ = writer.Close()
			return
		}
		if err := writer.Close(); err != nil {
			log.Printf("Could not finish gzip result for job %s: %v", shortID(jobID.String()), err)
		}
		log.Printf("Served result for job %s status=%s revision=%d bytes=%d encoding=gzip", shortID(jobID.String()), job.JobStatus, job.Revision, len(document))
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(document)))
	if _, err := w.Write(document); err != nil {
		log.Printf("Could not write result for job %s: %v", shortID(jobID.String()), err)
	}
	log.Printf("Served result for job %s status=%s revision=%d bytes=%d encoding=identity", shortID(jobID.String()), job.JobStatus, job.Revision, len(document))
}

func jobStatusResponseFor(sessionID string, job db.UserScriptJob) jobStatusResponse {
	response := jobStatusResponse{
		JobID:           job.JobID.String(),
		ClientRequestID: job.ClientRequestID,
		JobStatus:       job.JobStatus.String(),
		Version:         job.Revision,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
		CompletedAt:     job.CompletedAt,
		StatusURL:       jobStatusPath(sessionID, job.JobID),
	}
	if job.JobStatus == db.JobSucceeded {
		response.ResultURL = jobResultPath(sessionID, job.JobID)
	}
	if job.JobStatus == db.JobFailed {
		response.Error = &apiError{Code: "execution_failed", Message: job.Output}
	}
	return response
}

func writeSubmitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidClientRequestID):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, ErrCodeTooLarge):
		writeAPIError(w, http.StatusRequestEntityTooLarge, "code_too_large", err.Error())
	case errors.Is(err, ErrSessionBusy):
		writeAPIError(w, http.StatusConflict, "session_busy", err.Error())
	case errors.Is(err, ErrAgentPending):
		writeAPIError(w, http.StatusConflict, "agent_request_pending", err.Error())
	case errors.Is(err, ErrExecuteRequestExpired):
		writeAPIError(w, http.StatusGone, "execute_request_expired", err.Error())
	case errors.Is(err, db.ErrClientRequestConflict):
		writeAPIError(w, http.StatusConflict, "idempotency_conflict", err.Error())
	case errors.Is(err, ErrSessionClosed):
		writeAPIError(w, http.StatusServiceUnavailable, "session_closed", err.Error())
	default:
		log.Printf("Could not submit job: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to submit job")
	}
}

func writeAPIError(w http.ResponseWriter, status int, code string, message string) {
	writeJSONStatus(w, status, struct {
		Error apiError `json:"error"`
	}{
		Error: apiError{Code: code, Message: message},
	})
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("Could not write JSON response: %v", err)
	}
}

func gzipPreference(header string) (bool, bool) {
	if strings.TrimSpace(header) == "" {
		return false, false
	}

	gzipQuality := -1.0
	wildcardQuality := -1.0
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		coding := strings.ToLower(strings.TrimSpace(fields[0]))
		if coding == "" {
			continue
		}

		quality := 1.0
		for _, parameter := range fields[1:] {
			parameter = strings.TrimSpace(parameter)
			if !strings.HasPrefix(parameter, "q=") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(parameter, "q=")), 64)
			if err != nil {
				quality = 0
				break
			}
			quality = parsed
		}

		switch coding {
		case "gzip":
			gzipQuality = quality
		case "*":
			wildcardQuality = quality
		}
	}

	if gzipQuality >= 0 {
		return gzipQuality > 0, gzipQuality == 0
	}
	if wildcardQuality >= 0 {
		return wildcardQuality > 0, wildcardQuality == 0
	}
	return false, false
}

func ifNoneMatch(header string, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}
