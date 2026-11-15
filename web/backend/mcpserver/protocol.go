package mcpserver

import "errors"

const (
	MessageSessionReady    = "session.ready"
	MessageExecuteRequest  = "execute.request"
	MessageExecuteRejected = "execute.rejected"
	MessageRunStatus       = "run.status"
	MessageMCPStatus       = "mcp.status"
)

const (
	MCPStatusWaiting = "waiting"
	MCPStatusActive  = "active"
	MCPStatusIdle    = "idle"
)

var (
	ErrNoFrontend             = errors.New("no frontend is connected to this session")
	ErrSessionBusy            = errors.New("the editor is already running a request")
	ErrAgentPending           = errors.New("an agent request is already pending")
	ErrSessionClosed          = errors.New("session is closed")
	ErrExecuteTimeout         = errors.New("timed out waiting for the frontend to finish the run")
	ErrExecuteRequestExpired  = errors.New("the execute request is no longer pending")
	ErrInvalidClientRequestID = errors.New("client_request_id must be between 1 and 128 characters")
	ErrCodeTooLarge           = errors.New("code exceeds the 1048576 byte limit")
	ErrBackpressure           = errors.New("websocket send queue is full")
)

type SessionInfo struct {
	SessionID string `json:"session_id"`
	WSPath    string `json:"ws_path"`
	MCPPath   string `json:"mcp_path"`
}

type socketMessage struct {
	Type            string         `json:"type"`
	SessionID       string         `json:"session_id,omitempty"`
	RequestID       string         `json:"request_id,omitempty"`
	ClientRequestID string         `json:"client_request_id,omitempty"`
	JobID           string         `json:"job_id,omitempty"`
	Code            string         `json:"code,omitempty"`
	Error           string         `json:"error,omitempty"`
	JobStatus       string         `json:"job_status,omitempty"`
	Version         int64          `json:"version,omitempty"`
	ResultURL       string         `json:"result_url,omitempty"`
	ActiveRun       *activeRunInfo `json:"active_run,omitempty"`
	LastJob         *jobStatusInfo `json:"last_job,omitempty"`
	MCPStatus       string         `json:"mcp_status,omitempty"`
	MCPLastSeen     string         `json:"mcp_last_seen,omitempty"`
}

type activeRunInfo struct {
	RequestID       string `json:"request_id"`
	ClientRequestID string `json:"client_request_id,omitempty"`
	JobID           string `json:"job_id,omitempty"`
	JobStatus       string `json:"job_status,omitempty"`
	Version         int64  `json:"version"`
}

type jobStatusInfo struct {
	JobID           string `json:"job_id"`
	ClientRequestID string `json:"client_request_id,omitempty"`
	JobStatus       string `json:"job_status"`
	Version         int64  `json:"version"`
	ResultURL       string `json:"result_url,omitempty"`
	Error           string `json:"error,omitempty"`
}
