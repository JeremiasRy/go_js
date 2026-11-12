package mcpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go_js_demo/worker"
)

const (
	DefaultSessionTTL      = 30 * time.Minute
	sessionCleanupInterval = time.Minute
	jobBindingRetention    = 10 * time.Minute
)

type jobBinding struct {
	session *Session
	boundAt time.Time
}

type Manager struct {
	store JobStore
	work  chan<- worker.HeartbeatSignal
	ttl   time.Duration

	upgrader websocket.Upgrader

	mu       sync.Mutex
	sessions map[string]*Session
	jobs     map[uuid.UUID]jobBinding
	stop     chan struct{}
	stopOnce sync.Once
}

func NewManager(store JobStore, work chan<- worker.HeartbeatSignal, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}

	manager := &Manager{
		store:    store,
		work:     work,
		ttl:      ttl,
		sessions: make(map[string]*Session),
		jobs:     make(map[uuid.UUID]jobBinding),
		stop:     make(chan struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     sameOrigin,
		},
	}

	go manager.cleanupLoop()
	return manager
}

func (m *Manager) HandleWS(w http.ResponseWriter, r *http.Request) {
	sessionID, err := canonicalSessionID(r.PathValue("session_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := m.getOrCreateSession(sessionID)
	session.touch()

	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Session %s websocket upgrade failed: %v", shortID(sessionID), err)
		return
	}

	attached := session.attach(conn)
	if attached == nil {
		return
	}
	session.readLoop(attached)
}

type createSessionRequest struct {
	SessionID string `json:"session_id"`
	ClientID  string `json:"client_id"`
}

func (m *Manager) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestedID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if requestedID == "" {
		requestedID = strings.TrimSpace(r.URL.Query().Get("client_id"))
	}
	if r.Body != nil {
		var request createSessionRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&request); err != nil && err != io.EOF {
			http.Error(w, "invalid session request", http.StatusBadRequest)
			return
		}
		if requestedID == "" {
			requestedID = strings.TrimSpace(request.SessionID)
		}
		if requestedID == "" {
			requestedID = strings.TrimSpace(request.ClientID)
		}
	}

	var (
		info SessionInfo
		err  error
	)
	if requestedID == "" {
		info = m.CreateSession()
	} else {
		info, err = m.EnsureSession(requestedID)
		if err != nil {
			http.Error(w, "invalid session id", http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(info)
}

func (m *Manager) CreateSession() SessionInfo {
	info, _ := m.EnsureSession(uuid.NewString())
	return info
}

func (m *Manager) CreateSessionWithID(sessionID string) (SessionInfo, error) {
	return m.EnsureSession(sessionID)
}

func (m *Manager) EnsureSession(sessionID string) (SessionInfo, error) {
	id, err := canonicalSessionID(sessionID)
	if err != nil {
		return SessionInfo{}, err
	}

	m.getOrCreateSession(id)
	return sessionInfo(id), nil
}

func (m *Manager) getOrCreateSession(sessionID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session := m.sessions[sessionID]; session != nil {
		return session
	}

	session := newSession(sessionID, m, m.store, m.work)
	m.sessions[sessionID] = session
	return session
}

func sessionInfo(sessionID string) SessionInfo {
	return SessionInfo{
		SessionID: sessionID,
		WSPath:    "/ws/" + sessionID,
		MCPPath:   "/session/" + sessionID + "/mcp",
	}
}

func (m *Manager) HandleMCP(w http.ResponseWriter, r *http.Request) {
	sessionID, err := canonicalSessionID(r.PathValue("session_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := m.getOrCreateSession(sessionID)
	session.touch()
	if r.Method == http.MethodPost && requestHasJSONRPCMethod(r) {
		session.beginMCPRequest()
		defer session.endMCPRequest()
	}
	session.mcp.ServeHTTP(w, r)
}

func (m *Manager) DeliverJobResult(jobID uuid.UUID, status string, result string) {
	m.mu.Lock()
	binding, ok := m.jobs[jobID]
	if ok {
		delete(m.jobs, jobID)
	}
	m.mu.Unlock()

	if !ok {
		return
	}

	binding.session.completeJob(jobID, status, result)
}

func (m *Manager) bindJob(session *Session, jobID uuid.UUID) {
	m.mu.Lock()
	m.jobs[jobID] = jobBinding{
		session: session,
		boundAt: time.Now(),
	}
	m.mu.Unlock()
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

func (m *Manager) cleanup() {
	now := time.Now()

	m.mu.Lock()
	expired := make([]*Session, 0)
	for id, session := range m.sessions {
		if session.isConnected() {
			continue
		}
		if now.Sub(session.lastSeenAt()) < m.ttl {
			continue
		}

		delete(m.sessions, id)
		expired = append(expired, session)
	}

	for jobID, binding := range m.jobs {
		if now.Sub(binding.boundAt) > jobBindingRetention {
			delete(m.jobs, jobID)
		}
	}
	m.mu.Unlock()

	for _, session := range expired {
		session.close()
	}
}

func (m *Manager) Close() {
	m.stopOnce.Do(func() {
		close(m.stop)

		m.mu.Lock()
		sessions := make([]*Session, 0, len(m.sessions))
		for _, session := range m.sessions {
			sessions = append(sessions, session)
		}
		m.sessions = make(map[string]*Session)
		m.jobs = make(map[uuid.UUID]jobBinding)
		m.mu.Unlock()

		for _, session := range sessions {
			session.close()
		}
	})
}

func requestHasJSONRPCMethod(r *http.Request) bool {
	if r.Body == nil {
		return false
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxMessageSize+1))
	if err != nil {
		return false
	}

	if len(body) > maxMessageSize {
		r.Body = struct {
			io.Reader
			io.Closer
		}{
			Reader: io.MultiReader(bytes.NewReader(body), r.Body),
			Closer: r.Body,
		}
		return false
	}

	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))

	var single struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &single); err == nil && single.Method != "" {
		return true
	}

	var batch []struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &batch); err == nil {
		for _, message := range batch {
			if message.Method != "" {
				return true
			}
		}
	}

	return false
}

func canonicalSessionID(raw string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Host, r.Host)
}
