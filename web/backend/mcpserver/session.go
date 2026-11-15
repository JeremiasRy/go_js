package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go_js_demo/db"
	"go_js_demo/worker"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	maxMessageSize      = 2 << 20
	maxCodeSize         = 1 << 20
	maxClientRequestID  = 128
	writeWait           = 10 * time.Second
	pongWait            = 60 * time.Second
	pingPeriod          = 30 * time.Second
	jobInsertTimeout    = 5 * time.Second
	executeTimeout      = 120 * time.Second
	lastResultRetention = 5 * time.Minute
	outgoingQueueSize   = 64
)

var mcpIdleWindow = 60 * time.Second

type JobStore interface {
	InsertNewJob(sessionID, clientRequestID, src string, ctx context.Context) (*db.UserScriptJob, bool, error)
	GetJobsBySessionID(sessionID string, ctx context.Context) ([]db.UserScriptJob, error)
	GetRandomJob(ctx context.Context) (*db.UserScriptJob, error)
	GetJobBySessionAndID(sessionID string, jobID uuid.UUID, ctx context.Context) (*db.UserScriptJob, error)
	GetJobResultBySessionAndID(sessionID string, jobID uuid.UUID, ctx context.Context) (*db.UserScriptJob, error)
	GetJobBySessionAndClientRequestID(sessionID, clientRequestID string, ctx context.Context) (*db.UserScriptJob, error)
}

type Session struct {
	id    string
	mgr   *Manager
	store JobStore
	work  chan<- worker.HeartbeatSignal
	mcp   http.Handler

	submitMu sync.Mutex

	mu         sync.Mutex
	conns      map[*wsConn]struct{}
	active     *activeRun
	pendingMCP *pendingRun
	lastJob    *completedRun
	lastSeen   time.Time
	closed     bool

	mcpActive    bool
	mcpInFlight  int
	mcpLastSeen  time.Time
	mcpIdleTimer *time.Timer
}

type activeRun struct {
	requestID       string
	clientRequestID string
	jobID           uuid.UUID
	status          db.JobStatus
	revision        int64
	waiter          chan runOutcome
}

type pendingRun struct {
	requestID  string
	code       string
	waiter     chan runOutcome
	rejections map[*wsConn]string
}

type completedRun struct {
	jobID           uuid.UUID
	requestID       string
	clientRequestID string
	status          db.JobStatus
	revision        int64
	output          string
	errorMessage    string
	completedAt     time.Time
}

type runOutcome struct {
	JobID        uuid.UUID
	Status       db.JobStatus
	Output       string
	ErrorMessage string
	Revision     int64
	err          error
}

type SubmitRequest struct {
	ClientRequestID  string
	ExecuteRequestID string
	Code             string
}

type SubmitResult struct {
	Job       db.UserScriptJob
	RequestID string
	Created   bool
}

type wsConn struct {
	conn     *websocket.Conn
	out      chan []byte
	done     chan struct{}
	stopOnce sync.Once
}

func newWebsocketConn(conn *websocket.Conn) *wsConn {
	return &wsConn{
		conn: conn,
		out:  make(chan []byte, outgoingQueueSize),
		done: make(chan struct{}),
	}
}

func (c *wsConn) stop() {
	c.stopOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func newSession(id string, mgr *Manager, store JobStore, work chan<- worker.HeartbeatSignal) *Session {
	s := &Session{
		id:       id,
		mgr:      mgr,
		store:    store,
		work:     work,
		conns:    make(map[*wsConn]struct{}),
		lastSeen: time.Now(),
	}
	s.mcp = newMCPHandler(s)
	return s
}

func (s *Session) attach(raw *websocket.Conn) *wsConn {
	conn := newWebsocketConn(raw)
	now := time.Now()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		conn.stop()
		return nil
	}

	s.conns[conn] = struct{}{}
	s.lastSeen = now

	ready := socketMessage{
		Type:      MessageSessionReady,
		SessionID: s.id,
	}
	if s.mcpActive {
		ready.MCPStatus = MCPStatusActive
	} else if !s.mcpLastSeen.IsZero() {
		ready.MCPStatus = MCPStatusIdle
	} else {
		ready.MCPStatus = MCPStatusWaiting
	}
	if !s.mcpLastSeen.IsZero() {
		ready.MCPLastSeen = s.mcpLastSeen.UTC().Format(time.RFC3339)
	}
	if s.active != nil {
		ready.ActiveRun = &activeRunInfo{
			RequestID:       s.active.requestID,
			ClientRequestID: s.active.clientRequestID,
			JobID:           jobIDString(s.active.jobID),
			JobStatus:       s.active.status.String(),
			Version:         s.active.revision,
		}
	} else if s.lastJob != nil && now.Sub(s.lastJob.completedAt) < lastResultRetention {
		lastJob := jobStatusInfoFor(s.id, *s.lastJob)
		ready.LastJob = &lastJob
	}

	s.enqueueLocked(conn, ready)
	if s.pendingMCP != nil {
		s.enqueueLocked(conn, socketMessage{
			Type:      MessageExecuteRequest,
			RequestID: s.pendingMCP.requestID,
			Code:      s.pendingMCP.code,
		})
	}
	s.mu.Unlock()

	go s.writePump(conn)
	return conn
}

func (s *Session) detach(conn *wsConn) {
	s.submitMu.Lock()
	s.mu.Lock()
	if _, ok := s.conns[conn]; !ok {
		s.mu.Unlock()
		s.submitMu.Unlock()
		conn.stop()
		return
	}

	delete(s.conns, conn)
	s.lastSeen = time.Now()

	var (
		pending    *pendingRun
		pendingErr error
	)
	if s.pendingMCP != nil {
		delete(s.pendingMCP.rejections, conn)
	}
	if len(s.conns) == 0 && s.active == nil {
		pending = s.pendingMCP
		pendingErr = ErrNoFrontend
		s.pendingMCP = nil
	} else if s.pendingMCP != nil && len(s.pendingMCP.rejections) >= len(s.conns) {
		pending = s.pendingMCP
		for _, reason := range pending.rejections {
			pendingErr = fmt.Errorf("%s", reason)
			break
		}
		s.pendingMCP = nil
	}
	s.mu.Unlock()
	s.submitMu.Unlock()

	conn.stop()
	if pending != nil {
		pending.waiter <- runOutcome{err: pendingErr}
	}
}

func (s *Session) readLoop(conn *wsConn) {
	defer s.detach(conn)

	conn.conn.SetReadLimit(maxMessageSize)
	_ = conn.conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.conn.SetPongHandler(func(string) error {
		s.touch()
		return conn.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
				log.Printf("Session %s websocket closed unexpectedly: %v", shortID(s.id), err)
			}
			return
		}

		s.touch()

		var message socketMessage
		if err := json.Unmarshal(data, &message); err != nil {
			log.Printf("Session %s sent malformed websocket message: %v", shortID(s.id), err)
			continue
		}

		if message.Type == MessageExecuteRejected {
			s.handleExecuteRejected(conn, message)
		}
	}
}

func (s *Session) writePump(conn *wsConn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.stop()
	}()

	for {
		select {
		case <-conn.done:
			return
		case data := <-conn.out:
			_ = conn.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Session) send(message socketMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sendDataLocked(data)
}

func (s *Session) sendDataLocked(data []byte) error {
	if len(s.conns) == 0 {
		return ErrNoFrontend
	}

	sent := false
	for conn := range s.conns {
		if err := s.enqueueDataLocked(conn, data); err == nil {
			sent = true
		}
	}
	if sent {
		return nil
	}
	return ErrBackpressure
}

func (s *Session) enqueueLocked(conn *wsConn, message socketMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	_ = s.enqueueDataLocked(conn, data)
}

func (s *Session) enqueueDataLocked(conn *wsConn, data []byte) error {
	select {
	case conn.out <- data:
		return nil
	case <-conn.done:
		return ErrNoFrontend
	default:
		return ErrBackpressure
	}
}

func (s *Session) Submit(ctx context.Context, req SubmitRequest) (SubmitResult, error) {
	clientRequestID := strings.TrimSpace(req.ClientRequestID)
	executeRequestID := strings.TrimSpace(req.ExecuteRequestID)
	if clientRequestID == "" || len(clientRequestID) > maxClientRequestID {
		return SubmitResult{}, ErrInvalidClientRequestID
	}
	if len(req.Code) > maxCodeSize {
		return SubmitResult{}, ErrCodeTooLarge
	}

	s.submitMu.Lock()
	defer s.submitMu.Unlock()

	var (
		requestID string
		waiter    chan runOutcome
	)

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return SubmitResult{}, ErrSessionClosed
	}
	if s.active != nil {
		if (executeRequestID != "" && s.active.requestID == executeRequestID) ||
			(executeRequestID == "" && s.active.clientRequestID == clientRequestID) {
			active := *s.active
			s.mu.Unlock()

			job, err := s.store.GetJobBySessionAndID(s.id, active.jobID, ctx)
			if err != nil {
				return SubmitResult{}, err
			}
			if job.Src != req.Code {
				return SubmitResult{}, db.ErrClientRequestConflict
			}
			return SubmitResult{Job: *job, RequestID: active.requestID, Created: false}, nil
		}
		s.mu.Unlock()
		return SubmitResult{}, ErrSessionBusy
	}

	if executeRequestID != "" {
		if s.pendingMCP == nil || s.pendingMCP.requestID != executeRequestID {
			s.mu.Unlock()

			job, err := s.store.GetJobBySessionAndClientRequestID(s.id, clientRequestID, ctx)
			if errors.Is(err, db.ErrJobNotFound) {
				return SubmitResult{}, ErrExecuteRequestExpired
			}
			if err != nil {
				return SubmitResult{}, err
			}
			if job.Src != req.Code {
				return SubmitResult{}, db.ErrClientRequestConflict
			}
			return SubmitResult{Job: *job, RequestID: executeRequestID, Created: false}, nil
		}
		requestID = s.pendingMCP.requestID
		waiter = s.pendingMCP.waiter
		s.pendingMCP = nil
	} else {
		if s.pendingMCP != nil {
			s.mu.Unlock()
			return SubmitResult{}, ErrAgentPending
		}
		requestID = uuid.NewString()
	}
	s.mu.Unlock()

	job, created, err := s.store.InsertNewJob(s.id, clientRequestID, req.Code, ctx)
	if err != nil {
		if waiter != nil {
			waiter <- runOutcome{err: err}
		}
		return SubmitResult{}, err
	}

	if job.JobStatus == db.JobSucceeded || job.JobStatus == db.JobFailed {
		outcome := runOutcome{
			JobID:    job.JobID,
			Status:   job.JobStatus,
			Output:   job.Output,
			Revision: job.Revision,
		}
		if job.JobStatus == db.JobFailed {
			if stored, resultErr := s.store.GetJobResultBySessionAndID(s.id, job.JobID, ctx); resultErr == nil {
				outcome.ErrorMessage = stored.Result
			}
		}
		if waiter != nil {
			waiter <- outcome
		}
		s.send(statusMessageForJob(s.id, *job, requestID))
		return SubmitResult{Job: *job, RequestID: requestID, Created: created}, nil
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		if waiter != nil {
			waiter <- runOutcome{err: ErrSessionClosed}
		}
		return SubmitResult{}, ErrSessionClosed
	}
	if s.lastJob != nil && s.lastJob.jobID == job.JobID {
		completed := *s.lastJob
		s.mu.Unlock()
		job.JobStatus = completed.status
		job.Revision = completed.revision
		if completed.status == db.JobSucceeded {
			job.Output = completed.output
		} else {
			job.Output = completed.errorMessage
		}
		if waiter != nil {
			waiter <- outcomeForCompleted(completed)
		}
		return SubmitResult{Job: *job, RequestID: requestID, Created: created}, nil
	}
	if s.active != nil {
		if s.active.jobID != job.JobID {
			s.mu.Unlock()
			if waiter != nil {
				waiter <- runOutcome{err: ErrSessionBusy}
			}
			return SubmitResult{}, ErrSessionBusy
		}
		if s.active.requestID == "" {
			s.active.requestID = requestID
		}
		if s.active.waiter == nil && waiter != nil {
			s.active.waiter = waiter
		}
		if job.Revision > s.active.revision {
			s.active.status = job.JobStatus
			s.active.revision = job.Revision
		}
	} else {
		s.active = &activeRun{
			requestID:       requestID,
			clientRequestID: clientRequestID,
			jobID:           job.JobID,
			status:          job.JobStatus,
			revision:        job.Revision,
			waiter:          waiter,
		}
	}
	status := s.active.status
	revision := s.active.revision
	s.mu.Unlock()

	s.mgr.bindJob(s, job.JobID)

	job.JobStatus = status
	job.Revision = revision
	if err := s.send(statusMessageForJob(s.id, *job, requestID)); err != nil {
		log.Printf("Session %s could not publish status for job %s: %v", shortID(s.id), shortID(job.JobID.String()), err)
	}

	select {
	case s.work <- worker.HeartbeatSignal{}:
	default:
	}

	return SubmitResult{Job: *job, RequestID: requestID, Created: created}, nil
}

func (s *Session) handleExecuteRejected(conn *wsConn, message socketMessage) {
	reason := strings.TrimSpace(message.Error)
	if reason == "" {
		reason = "frontend rejected the execute request"
	}

	s.submitMu.Lock()
	s.mu.Lock()
	pending := s.pendingMCP
	if pending == nil || pending.requestID != message.RequestID {
		s.mu.Unlock()
		s.submitMu.Unlock()
		return
	}
	if pending.rejections == nil {
		pending.rejections = map[*wsConn]string{}
	}
	pending.rejections[conn] = reason
	for rejectedConn := range pending.rejections {
		if _, connected := s.conns[rejectedConn]; !connected {
			delete(pending.rejections, rejectedConn)
		}
	}

	allRejected := len(s.conns) > 0 && len(pending.rejections) >= len(s.conns)
	if allRejected {
		s.pendingMCP = nil
	}
	s.mu.Unlock()
	s.submitMu.Unlock()

	if allRejected {
		pending.waiter <- runOutcome{err: fmt.Errorf("%s", reason)}
	}
}

func (s *Session) Run(ctx context.Context, code string) (runOutcome, error) {
	if len(code) > maxCodeSize {
		return runOutcome{}, ErrCodeTooLarge
	}

	s.submitMu.Lock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.submitMu.Unlock()
		return runOutcome{}, ErrSessionClosed
	}
	if len(s.conns) == 0 {
		s.mu.Unlock()
		s.submitMu.Unlock()
		return runOutcome{}, ErrNoFrontend
	}
	if s.active != nil || s.pendingMCP != nil {
		s.mu.Unlock()
		s.submitMu.Unlock()
		return runOutcome{}, ErrSessionBusy
	}

	requestID := uuid.NewString()
	waiter := make(chan runOutcome, 1)
	s.pendingMCP = &pendingRun{
		requestID:  requestID,
		code:       code,
		waiter:     waiter,
		rejections: map[*wsConn]string{},
	}

	data, err := json.Marshal(socketMessage{
		Type:      MessageExecuteRequest,
		RequestID: requestID,
		Code:      code,
	})
	if err != nil {
		s.pendingMCP = nil
		s.mu.Unlock()
		s.submitMu.Unlock()
		return runOutcome{}, err
	}
	if err := s.sendDataLocked(data); err != nil {
		s.pendingMCP = nil
		s.mu.Unlock()
		s.submitMu.Unlock()
		return runOutcome{}, err
	}
	s.mu.Unlock()
	s.submitMu.Unlock()

	timer := time.NewTimer(executeTimeout)
	defer timer.Stop()

	select {
	case outcome := <-waiter:
		if outcome.err != nil {
			return runOutcome{}, outcome.err
		}
		return outcome, nil
	case <-ctx.Done():
		s.clearPendingMCP(requestID)
		return runOutcome{}, ctx.Err()
	case <-timer.C:
		s.clearPendingMCP(requestID)
		return runOutcome{}, ErrExecuteTimeout
	}
}

func (s *Session) applyJobUpdate(update worker.JobResult) {
	terminal := update.Status == db.JobSucceeded || update.Status == db.JobFailed
	var (
		waiter     chan runOutcome
		requestID  string
		clientID   string
		output     string
		errorText  string
		revision   int64
		status     db.JobStatus
		shouldSend bool
	)

	s.mu.Lock()
	if s.lastJob != nil && s.lastJob.jobID == update.JobID && s.lastJob.revision >= update.Revision {
		s.mu.Unlock()
		return
	}

	var matched *activeRun
	if s.active != nil && s.active.jobID == update.JobID {
		matched = s.active
	}

	if terminal {
		if matched != nil {
			waiter = matched.waiter
			requestID = matched.requestID
			clientID = matched.clientRequestID
			s.active = nil
		}
		if update.ClientRequestID != "" {
			clientID = update.ClientRequestID
		}
		if clientID == "" && s.lastJob != nil && s.lastJob.jobID == update.JobID {
			clientID = s.lastJob.clientRequestID
		}

		completed := completedRun{
			jobID:           update.JobID,
			requestID:       requestID,
			clientRequestID: clientID,
			status:          update.Status,
			revision:        update.Revision,
			output:          update.Output,
			errorMessage:    update.ErrorMessage,
			completedAt:     time.Now(),
		}
		s.lastJob = &completed
		status = update.Status
		revision = update.Revision
		output = update.Output
		errorText = update.ErrorMessage
		shouldSend = true
	} else if update.Status == db.JobProcessing {
		if matched == nil {
			s.active = &activeRun{
				clientRequestID: update.ClientRequestID,
				jobID:           update.JobID,
				status:          update.Status,
				revision:        update.Revision,
			}
			matched = s.active
		} else if update.Revision > matched.revision {
			matched.status = update.Status
			matched.revision = update.Revision
		}
		requestID = matched.requestID
		clientID = matched.clientRequestID
		if clientID == "" {
			clientID = update.ClientRequestID
		}
		status = matched.status
		revision = matched.revision
		shouldSend = true
	}
	s.mu.Unlock()

	if !shouldSend {
		return
	}

	message := socketMessage{
		Type:            MessageRunStatus,
		RequestID:       requestID,
		ClientRequestID: clientID,
		JobID:           update.JobID.String(),
		JobStatus:       status.String(),
		Version:         revision,
	}
	if terminal {
		if status == db.JobSucceeded {
			message.ResultURL = jobResultPath(s.id, update.JobID)
		} else {
			message.Error = errorText
		}
	}

	if err := s.send(message); err != nil {
		log.Printf("Session %s could not deliver status for job %s: %v", shortID(s.id), shortID(update.JobID.String()), err)
	}

	if waiter != nil {
		waiter <- runOutcome{
			JobID:        update.JobID,
			Status:       status,
			Output:       output,
			ErrorMessage: errorText,
			Revision:     revision,
		}
	}
}

func (s *Session) clearPendingMCP(requestID string) {
	s.submitMu.Lock()
	s.mu.Lock()
	if s.pendingMCP != nil && s.pendingMCP.requestID == requestID {
		s.pendingMCP = nil
	}
	s.mu.Unlock()
	s.submitMu.Unlock()
}

func (s *Session) close() {
	s.submitMu.Lock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.submitMu.Unlock()
		return
	}
	s.closed = true

	conns := make([]*wsConn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.conns = make(map[*wsConn]struct{})
	pending := s.pendingMCP
	s.pendingMCP = nil
	active := s.active
	s.active = nil
	if s.mcpIdleTimer != nil {
		s.mcpIdleTimer.Stop()
		s.mcpIdleTimer = nil
	}
	s.mu.Unlock()
	s.submitMu.Unlock()

	for _, conn := range conns {
		conn.stop()
	}
	if pending != nil {
		pending.waiter <- runOutcome{err: ErrSessionClosed}
	}
	if active != nil && active.waiter != nil {
		active.waiter <- runOutcome{err: ErrSessionClosed}
	}
}

func (s *Session) beginMCPRequest() {
	now := time.Now()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mcpLastSeen = now
	s.mcpInFlight++
	if s.mcpIdleTimer != nil {
		s.mcpIdleTimer.Stop()
		s.mcpIdleTimer = nil
	}
	becameActive := !s.mcpActive
	s.mcpActive = true
	s.mu.Unlock()

	if becameActive {
		s.sendMCPStatus(MCPStatusActive, now)
	}
}

func (s *Session) endMCPRequest() {
	s.mu.Lock()
	if s.mcpInFlight > 0 {
		s.mcpInFlight--
	}
	if s.mcpInFlight == 0 && s.mcpActive && !s.closed {
		if s.mcpIdleTimer != nil {
			s.mcpIdleTimer.Stop()
		}
		s.mcpIdleTimer = time.AfterFunc(mcpIdleWindow, s.expireMCPActivity)
	}
	s.mu.Unlock()
}

func (s *Session) expireMCPActivity() {
	s.mu.Lock()
	if s.mcpInFlight > 0 || !s.mcpActive || s.closed {
		s.mu.Unlock()
		return
	}
	s.mcpActive = false
	s.mcpIdleTimer = nil
	lastSeen := s.mcpLastSeen
	s.mu.Unlock()

	s.sendMCPStatus(MCPStatusIdle, lastSeen)
}

func (s *Session) sendMCPStatus(status string, lastSeen time.Time) {
	message := socketMessage{
		Type:      MessageMCPStatus,
		MCPStatus: status,
	}
	if !lastSeen.IsZero() {
		message.MCPLastSeen = lastSeen.UTC().Format(time.RFC3339)
	}
	_ = s.send(message)
}

func (s *Session) touch() {
	s.mu.Lock()
	s.lastSeen = time.Now()
	s.mu.Unlock()
}

func (s *Session) isConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns) > 0
}

func (s *Session) lastSeenAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSeen
}

func jobIDString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func jobResultPath(sessionID string, jobID uuid.UUID) string {
	return "/session/" + sessionID + "/jobs/" + jobID.String() + "/result"
}

func jobStatusPath(sessionID string, jobID uuid.UUID) string {
	return "/session/" + sessionID + "/jobs/" + jobID.String()
}

func statusMessageForJob(sessionID string, job db.UserScriptJob, requestID string) socketMessage {
	message := socketMessage{
		Type:            MessageRunStatus,
		RequestID:       requestID,
		ClientRequestID: job.ClientRequestID,
		JobID:           job.JobID.String(),
		JobStatus:       job.JobStatus.String(),
		Version:         job.Revision,
	}
	if job.JobStatus == db.JobSucceeded {
		message.ResultURL = jobResultPath(sessionID, job.JobID)
	}
	return message
}

func jobStatusInfoFor(sessionID string, job completedRun) jobStatusInfo {
	info := jobStatusInfo{
		JobID:           job.jobID.String(),
		ClientRequestID: job.clientRequestID,
		JobStatus:       job.status.String(),
		Version:         job.revision,
	}
	if job.status == db.JobSucceeded {
		info.ResultURL = jobResultPath(sessionID, job.jobID)
	} else {
		info.Error = job.errorMessage
	}
	return info
}

func outcomeForCompleted(job completedRun) runOutcome {
	return runOutcome{
		JobID:        job.jobID,
		Status:       job.status,
		Output:       job.output,
		ErrorMessage: job.errorMessage,
		Revision:     job.revision,
	}
}
