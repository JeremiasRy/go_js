package mcpserver

import "errors"

const (
	MessageSessionReady    = "session.ready"
	MessageExecuteRequest  = "execute.request"
	MessageExecuteRejected = "execute.rejected"
	MessageRunSubmit       = "run.submit"
	MessageRunAccepted     = "run.accepted"
	MessageRunResult       = "run.result"
	MessageRunError        = "run.error"
	MessageMCPStatus       = "mcp.status"
)

const (
	MCPStatusWaiting = "waiting"
	MCPStatusActive  = "active"
	MCPStatusIdle    = "idle"
)

var (
	ErrNoFrontend     = errors.New("no frontend is connected to this session")
	ErrSessionBusy    = errors.New("the editor is already running a request")
	ErrSessionClosed  = errors.New("session is closed")
	ErrExecuteTimeout = errors.New("timed out waiting for the frontend to finish the run")
	ErrBackpressure   = errors.New("websocket send queue is full")
)

type SessionInfo struct {
	SessionID string `json:"session_id"`
	WSPath    string `json:"ws_path"`
	MCPPath   string `json:"mcp_path"`
}

type socketMessage struct {
	Type        string         `json:"type"`
	SessionID   string         `json:"session_id,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	JobID       string         `json:"job_id,omitempty"`
	Code        string         `json:"code,omitempty"`
	Error       string         `json:"error,omitempty"`
	JobStatus   string         `json:"job_status,omitempty"`
	Result      string         `json:"result,omitempty"`
	ActiveRun   *activeRunInfo `json:"active_run,omitempty"`
	MCPStatus   string         `json:"mcp_status,omitempty"`
	MCPLastSeen string         `json:"mcp_last_seen,omitempty"`
}

type activeRunInfo struct {
	RequestID string `json:"request_id"`
	JobID     string `json:"job_id,omitempty"`
}
