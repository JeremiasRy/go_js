package mcpserver

import (
	"context"
	"encoding/json"
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
	writeWait           = 10 * time.Second
	pongWait            = 60 * time.Second
	pingPeriod          = 30 * time.Second
	jobInsertTimeout    = 5 * time.Second
	workSignalTimeout   = 5 * time.Second
	executeTimeout      = 120 * time.Second
	lastResultRetention = 5 * time.Minute
	outgoingQueueSize   = 64
)

var mcpIdleWindow = 60 * time.Second

type JobStore interface {
	InsertNewJob(src string, ctx context.Context) (uuid.UUID, error)
	UpdateJob(status db.JobStatus, result string, jobID uuid.UUID, ctx context.Context) error
}

type Session struct {
	id    string
	mgr   *Manager
	store JobStore
	work  chan<- worker.HeartbeatSignal
	mcp   http.Handler

	mu             sync.Mutex
	conns          map[*wsConn]struct{}
	active         *activeRun
	pendingMCP     *pendingRun
	lastResult     *completedRun
	recentRequests map[string]time.Time
	lastSeen       time.Time
	closed         bool

	mcpActive    bool
	mcpInFlight  int
	mcpLastSeen  time.Time
	mcpIdleTimer *time.Timer
}

type activeRun struct {
	requestID string
	jobID     uuid.UUID
	waiter    chan runOutcome
}

type pendingRun struct {
	requestID string
	code      string
	waiter    chan runOutcome
}

type completedRun struct {
	message     socketMessage
	completedAt time.Time
}

type runOutcome struct {
	JobID  uuid.UUID
	Status string
	Result string
	err    error
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
		id:             id,
		mgr:            mgr,
		store:          store,
		work:           work,
		conns:          make(map[*wsConn]struct{}),
		recentRequests: make(map[string]time.Time),
		lastSeen:       time.Now(),
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
			RequestID: s.active.requestID,
			JobID:     jobIDString(s.active.jobID),
		}
	}

	s.enqueueLocked(conn, ready)
	if s.active == nil && s.lastResult != nil && now.Sub(s.lastResult.completedAt) < lastResultRetention {
		s.enqueueLocked(conn, s.lastResult.message)
	}
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
	s.mu.Lock()
	if _, ok := s.conns[conn]; !ok {
		s.mu.Unlock()
		conn.stop()
		return
	}

	delete(s.conns, conn)
	s.lastSeen = time.Now()

	var pending *pendingRun
	if len(s.conns) == 0 && s.active == nil {
		pending = s.pendingMCP
		s.pendingMCP = nil
	}
	s.mu.Unlock()

	conn.stop()
	if pending != nil {
		pending.waiter <- runOutcome{err: ErrNoFrontend}
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

		switch message.Type {
		case MessageRunSubmit:
			s.handleRunSubmit(message)
		case MessageExecuteRejected:
			s.handleExecuteRejected(message)
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

func (s *Session) handleRunSubmit(message socketMessage) {
	incomingID := strings.TrimSpace(message.RequestID)
	if len(message.Code) > maxCodeSize {
		s.sendRunError(runErrorID(incomingID), fmt.Sprintf("code exceeds the %d byte limit", maxCodeSize))
		return
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if incomingID != "" {
		if seenAt, ok := s.recentRequests[incomingID]; ok && time.Since(seenAt) < lastResultRetention {
			s.mu.Unlock()
			return
		}
		if s.active != nil && s.active.requestID == incomingID {
			s.mu.Unlock()
			return
		}
	}
	if s.active != nil {
		s.mu.Unlock()
		s.sendRunError(runErrorID(incomingID), ErrSessionBusy.Error())
		return
	}

	requestID := ""
	var waiter chan runOutcome
	if s.pendingMCP != nil {
		if incomingID == "" || incomingID != s.pendingMCP.requestID {
			s.mu.Unlock()
			s.sendRunError(runErrorID(incomingID), "an agent request is already pending")
			return
		}
		requestID = s.pendingMCP.requestID
		waiter = s.pendingMCP.waiter
		s.pendingMCP = nil
	} else {
		requestID = uuid.NewString()
	}

	now := time.Now()
	for id, seenAt := range s.recentRequests {
		if now.Sub(seenAt) >= lastResultRetention {
			delete(s.recentRequests, id)
		}
	}
	s.recentRequests[requestID] = now

	s.active = &activeRun{
		requestID: requestID,
		waiter:    waiter,
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), jobInsertTimeout)
	jobID, err := s.store.InsertNewJob(message.Code, ctx)
	cancel()
	if err != nil {
		s.failActive(requestID, fmt.Errorf("failed to queue job: %w", err))
		return
	}

	s.mu.Lock()
	if s.active != nil && s.active.requestID == requestID {
		s.active.jobID = jobID
	}
	s.mu.Unlock()

	s.mgr.bindJob(s, jobID)

	if err := s.send(socketMessage{
		Type:      MessageRunAccepted,
		RequestID: requestID,
		JobID:     jobID.String(),
	}); err != nil {
		log.Printf("Session %s could not acknowledge run %s: %v", shortID(s.id), shortID(requestID), err)
	}

	select {
	case s.work <- worker.HeartbeatSignal{}:
	case <-time.After(workSignalTimeout):
		s.failActiveWithJob(requestID, jobID, "worker queue is full")
	}
}

func (s *Session) handleExecuteRejected(message socketMessage) {
	s.mu.Lock()
	pending := s.pendingMCP
	if pending != nil && pending.requestID == message.RequestID {
		s.pendingMCP = nil
	} else {
		pending = nil
	}
	s.mu.Unlock()

	if pending != nil {
		reason := message.Error
		if reason == "" {
			reason = "frontend rejected the execute request"
		}
		pending.waiter <- runOutcome{err: fmt.Errorf("%s", reason)}
	}
}

func (s *Session) Run(ctx context.Context, code string) (runOutcome, error) {
	if len(code) > maxCodeSize {
		return runOutcome{}, fmt.Errorf("code exceeds the %d byte limit", maxCodeSize)
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return runOutcome{}, ErrSessionClosed
	}
	if len(s.conns) == 0 {
		s.mu.Unlock()
		return runOutcome{}, ErrNoFrontend
	}
	if s.active != nil || s.pendingMCP != nil {
		s.mu.Unlock()
		return runOutcome{}, ErrSessionBusy
	}

	requestID := uuid.NewString()
	waiter := make(chan runOutcome, 1)
	s.pendingMCP = &pendingRun{
		requestID: requestID,
		code:      code,
		waiter:    waiter,
	}

	data, err := json.Marshal(socketMessage{
		Type:      MessageExecuteRequest,
		RequestID: requestID,
		Code:      code,
	})
	if err != nil {
		s.pendingMCP = nil
		s.mu.Unlock()
		return runOutcome{}, err
	}
	if err := s.sendDataLocked(data); err != nil {
		s.pendingMCP = nil
		s.mu.Unlock()
		return runOutcome{}, err
	}
	s.mu.Unlock()

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

func (s *Session) completeJob(jobID uuid.UUID, status string, result string) {
	s.mu.Lock()
	if s.active == nil || s.active.jobID != jobID {
		s.mu.Unlock()
		return
	}

	active := s.active
	s.active = nil
	message := socketMessage{
		Type:      MessageRunResult,
		RequestID: active.requestID,
		JobID:     jobID.String(),
		JobStatus: status,
		Result:    result,
	}
	s.lastResult = &completedRun{
		message:     message,
		completedAt: time.Now(),
	}
	s.mu.Unlock()

	if err := s.send(message); err != nil {
		log.Printf("Session %s could not deliver result for job %s: %v", shortID(s.id), shortID(jobID.String()), err)
	}

	if active.waiter != nil {
		active.waiter <- runOutcome{
			JobID:  jobID,
			Status: status,
			Result: result,
		}
	}
}

func (s *Session) failActive(requestID string, err error) {
	s.mu.Lock()
	if s.active == nil || s.active.requestID != requestID {
		s.mu.Unlock()
		return
	}
	active := s.active
	s.active = nil
	s.mu.Unlock()

	s.sendRunError(requestID, err.Error())
	if active.waiter != nil {
		active.waiter <- runOutcome{err: err}
	}
}

func (s *Session) failActiveWithJob(requestID string, jobID uuid.UUID, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), jobInsertTimeout)
	_ = s.store.UpdateJob(db.JobFailed, reason, jobID, ctx)
	cancel()
	s.failActive(requestID, fmt.Errorf("%s", reason))
}

func (s *Session) sendRunError(requestID string, reason string) {
	_ = s.send(socketMessage{
		Type:      MessageRunError,
		RequestID: requestID,
		Error:     reason,
	})
}

func (s *Session) clearPendingMCP(requestID string) {
	s.mu.Lock()
	if s.pendingMCP != nil && s.pendingMCP.requestID == requestID {
		s.pendingMCP = nil
	}
	s.mu.Unlock()
}

func (s *Session) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
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

func runErrorID(incoming string) string {
	if strings.TrimSpace(incoming) != "" {
		return incoming
	}
	return uuid.NewString()
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
