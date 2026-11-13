package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go_js_demo/db"
	"go_js_demo/worker"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeStore struct {
	mu   sync.Mutex
	jobs map[uuid.UUID]db.UserScriptJob
}

func newFakeStore() *fakeStore {
	return &fakeStore{jobs: make(map[uuid.UUID]db.UserScriptJob)}
}

func (f *fakeStore) InsertNewJob(sessionID string, src string, _ context.Context) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job := db.UserScriptJob{
		JobId:     uuid.New(),
		SessionID: sessionID,
		Src:       src,
		JobStatus: db.JobPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.jobs[job.JobId] = job
	return job.JobId, nil
}

func (f *fakeStore) UpdateJob(status db.JobStatus, result string, jobID uuid.UUID, _ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	job.JobStatus = status
	job.Result = result
	job.UpdatedAt = time.Now()
	f.jobs[jobID] = job
	return nil
}

func (f *fakeStore) GetJobsBySessionID(sessionID string, _ context.Context) ([]db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	jobs := make([]db.UserScriptJob, 0)
	for _, job := range f.jobs {
		if job.SessionID == sessionID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs, nil
}

func (f *fakeStore) GetRandomJob(_ context.Context) (*db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, job := range f.jobs {
		if job.JobStatus == db.JobFailed {
			continue
		}
		jobCopy := job
		return &jobCopy, nil
	}
	return nil, db.ErrNoJobs
}

func newTestServer(t *testing.T, manager *Manager) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", manager.HandleCreateSession)
	mux.HandleFunc("GET /ws/{session_id}", manager.HandleWS)
	mux.HandleFunc("GET /session/{session_id}/jobs", manager.HandleSessionJobs)
	mux.HandleFunc("GET /scripts/random", manager.HandleRandomJob)
	mux.HandleFunc("/session/{session_id}/mcp", manager.HandleMCP)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func dialSession(t *testing.T, server *httptest.Server, manager *Manager) (*websocket.Conn, string) {
	t.Helper()

	session := manager.CreateSession()
	conn, ready := dialSessionByID(t, server, session.SessionID)
	if ready.Type != MessageSessionReady {
		t.Fatalf("expected %q, got %q", MessageSessionReady, ready.Type)
	}
	if ready.SessionID != session.SessionID {
		t.Fatalf("expected session %q, got %q", session.SessionID, ready.SessionID)
	}
	if ready.MCPStatus != MCPStatusWaiting {
		t.Fatalf("expected MCP status %q, got %q", MCPStatusWaiting, ready.MCPStatus)
	}

	return conn, session.SessionID
}

func dialSessionByID(t *testing.T, server *httptest.Server, sessionID string) (*websocket.Conn, socketMessage) {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/" + sessionID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return conn, readSocketMessage(t, conn)
}

func readSocketMessage(t *testing.T, conn *websocket.Conn) socketMessage {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}

	var message socketMessage
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("decode websocket message: %v", err)
	}
	return message
}

func writeSocketMessage(t *testing.T, conn *websocket.Conn, message socketMessage) {
	t.Helper()

	if err := conn.WriteJSON(message); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}
}

func encodedResult(output string) string {
	payload, _ := json.Marshal(map[string]any{
		"format_version": 2,
		"output":         output,
		"debug_info": map[string]any{
			"chunks": []any{},
		},
		"ast": map[string]any{},
		"execution": map[string]any{
			"schema_version": 1,
			"status":         "not_requested",
			"frames":         []any{},
			"truncated":      false,
			"total_frames":   0,
			"dropped_frames": 0,
			"duration_ms":    0,
			"limits": map[string]any{
				"max_frames":      10000,
				"max_stack":       64,
				"max_queue_items": 32,
				"max_timers":      16,
				"max_value_chars": 160,
			},
		},
	})
	return base64.StdEncoding.EncodeToString(payload)
}

func TestWebSocketCreatesSessionForClientID(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	clientID := uuid.NewString()
	conn, ready := dialSessionByID(t, server, clientID)
	if ready.Type != MessageSessionReady {
		t.Fatalf("expected %q, got %q", MessageSessionReady, ready.Type)
	}
	if ready.SessionID != clientID {
		t.Fatalf("expected client session %q, got %q", clientID, ready.SessionID)
	}
	_ = conn.Close()
}

func TestCreateSessionEndpointUsesClientID(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	clientID := uuid.NewString()
	body := strings.NewReader(`{"session_id":"` + clientID + `"}`)
	response, err := http.Post(server.URL+"/session", "application/json", body)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	var first SessionInfo
	if err := json.NewDecoder(response.Body).Decode(&first); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if first.SessionID != clientID {
		t.Fatalf("expected session %q, got %q", clientID, first.SessionID)
	}
	if first.WSPath != "/ws/"+clientID || first.MCPPath != "/session/"+clientID+"/mcp" {
		t.Fatalf("unexpected session paths: %#v", first)
	}

	second, err := manager.EnsureSession(clientID)
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if second != first {
		t.Fatalf("expected the same session to be reused, got %#v and %#v", first, second)
	}
}

func TestSessionJobsListsOnlyClientScripts(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	clientID := uuid.NewString()
	otherClientID := uuid.NewString()
	ctx := context.Background()

	if _, err := store.InsertNewJob(clientID, "console.log(1)", ctx); err != nil {
		t.Fatalf("insert first job: %v", err)
	}
	if _, err := store.InsertNewJob(clientID, "console.log(2)", ctx); err != nil {
		t.Fatalf("insert second job: %v", err)
	}
	if _, err := store.InsertNewJob(otherClientID, "console.log(3)", ctx); err != nil {
		t.Fatalf("insert other job: %v", err)
	}

	response, err := http.Get(server.URL + "/session/" + clientID + "/jobs")
	if err != nil {
		t.Fatalf("list scripts: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	var jobs []scriptSummary
	if err := json.NewDecoder(response.Body).Decode(&jobs); err != nil {
		t.Fatalf("decode scripts: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 scripts, got %d", len(jobs))
	}
	for _, job := range jobs {
		if job.SessionID != clientID {
			t.Fatalf("expected scripts for %q, got %q", clientID, job.SessionID)
		}
		if strings.Contains(job.Src, "console.log(3)") {
			t.Fatal("listed a script from another client")
		}
	}
}

func TestSessionJobsRejectsInvalidSessionID(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	response, err := http.Get(server.URL + "/session/not-a-session/jobs")
	if err != nil {
		t.Fatalf("list scripts: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.StatusCode)
	}
}

func TestRandomJobReturnsStoredScript(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	ctx := context.Background()

	empty, err := http.Get(server.URL + "/scripts/random")
	if err != nil {
		t.Fatalf("get random script: %v", err)
	}
	empty.Body.Close()
	if empty.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when no scripts exist, got %d", empty.StatusCode)
	}

	failedJobID, err := store.InsertNewJob(uuid.NewString(), "console.log('failed')", ctx)
	if err != nil {
		t.Fatalf("insert failed job: %v", err)
	}
	if err := store.UpdateJob(db.JobFailed, "nope", failedJobID, ctx); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	failedOnly, err := http.Get(server.URL + "/scripts/random")
	if err != nil {
		t.Fatalf("get random script: %v", err)
	}
	failedOnly.Body.Close()
	if failedOnly.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when only failed scripts exist, got %d", failedOnly.StatusCode)
	}

	sessionID := uuid.NewString()
	if _, err := store.InsertNewJob(sessionID, "console.log('random')", ctx); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	response, err := http.Get(server.URL + "/scripts/random")
	if err != nil {
		t.Fatalf("get random script: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	var job scriptSummary
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		t.Fatalf("decode random script: %v", err)
	}
	if job.Src != "console.log('random')" {
		t.Fatalf("unexpected random script: %q", job.Src)
	}
	if job.JobStatus != db.JobPending {
		t.Fatalf("expected pending status, got %q", job.JobStatus)
	}
}

func TestMultipleTabsShareSession(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	session, err := manager.EnsureSession(uuid.NewString())
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	first, firstReady := dialSessionByID(t, server, session.SessionID)
	if firstReady.SessionID != session.SessionID {
		t.Fatalf("expected session %q, got %q", session.SessionID, firstReady.SessionID)
	}

	second, secondReady := dialSessionByID(t, server, session.SessionID)
	if secondReady.SessionID != session.SessionID {
		t.Fatalf("expected session %q, got %q", session.SessionID, secondReady.SessionID)
	}

	writeSocketMessage(t, first, socketMessage{
		Type: MessageRunSubmit,
		Code: "console.log(4)",
	})

	firstAccepted := readSocketMessage(t, first)
	secondAccepted := readSocketMessage(t, second)
	if firstAccepted.Type != MessageRunAccepted || secondAccepted.Type != MessageRunAccepted {
		t.Fatalf("expected both tabs to receive %q, got %q and %q", MessageRunAccepted, firstAccepted.Type, secondAccepted.Type)
	}
	if firstAccepted.JobID != secondAccepted.JobID {
		t.Fatalf("expected a shared job, got %q and %q", firstAccepted.JobID, secondAccepted.JobID)
	}

	jobID, err := uuid.Parse(firstAccepted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	raw := encodedResult("4")
	manager.DeliverJobResult(jobID, "Success", raw)

	firstResult := readSocketMessage(t, first)
	secondResult := readSocketMessage(t, second)
	if firstResult.Type != MessageRunResult || secondResult.Type != MessageRunResult {
		t.Fatalf("expected both tabs to receive %q, got %q and %q", MessageRunResult, firstResult.Type, secondResult.Type)
	}
	if firstResult.JobID != jobID.String() || secondResult.JobID != jobID.String() {
		t.Fatalf("expected both tabs to receive job %q", jobID)
	}
}

func TestWebSocketRunFlow(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	clientID := uuid.NewString()
	writeSocketMessage(t, conn, socketMessage{
		Type:      MessageRunSubmit,
		RequestID: clientID,
		Code:      "console.log(2)",
	})

	accepted := readSocketMessage(t, conn)
	if accepted.Type != MessageRunAccepted {
		t.Fatalf("expected %q, got %q (%s)", MessageRunAccepted, accepted.Type, accepted.Error)
	}
	if _, err := uuid.Parse(accepted.RequestID); err != nil {
		t.Fatalf("expected backend-generated request id, got %q", accepted.RequestID)
	}
	if accepted.RequestID == clientID {
		t.Fatal("manual runs must not reuse a client-supplied request id")
	}
	requestID := accepted.RequestID

	jobID, err := uuid.Parse(accepted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}

	store.mu.Lock()
	job, ok := store.jobs[jobID]
	store.mu.Unlock()
	if !ok {
		t.Fatalf("job %s was not stored", jobID)
	}
	if job.SessionID != sessionID {
		t.Fatalf("expected job session %q, got %q", sessionID, job.SessionID)
	}

	raw := encodedResult("2")
	manager.DeliverJobResult(jobID, "Success", raw)

	result := readSocketMessage(t, conn)
	if result.Type != MessageRunResult {
		t.Fatalf("expected %q, got %q", MessageRunResult, result.Type)
	}
	if result.RequestID != requestID {
		t.Fatalf("expected request %q, got %q", requestID, result.RequestID)
	}
	if result.JobID != jobID.String() {
		t.Fatalf("expected job %q, got %q", jobID, result.JobID)
	}
	if result.JobStatus != "Success" {
		t.Fatalf("expected success, got %q", result.JobStatus)
	}
	if result.Result != raw {
		t.Fatalf("unexpected result payload")
	}
}

func TestReconnectReceivesLastResult(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	writeSocketMessage(t, conn, socketMessage{
		Type: MessageRunSubmit,
		Code: "console.log(3)",
	})

	accepted := readSocketMessage(t, conn)
	requestID := accepted.RequestID
	jobID, err := uuid.Parse(accepted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}

	_ = conn.Close()
	time.Sleep(20 * time.Millisecond)

	raw := encodedResult("3")
	manager.DeliverJobResult(jobID, "Success", raw)

	reconnected, ready := dialSessionByID(t, server, sessionID)
	if ready.Type != MessageSessionReady {
		t.Fatalf("expected %q, got %q", MessageSessionReady, ready.Type)
	}
	if ready.ActiveRun != nil {
		t.Fatalf("expected no active run, got %#v", ready.ActiveRun)
	}

	result := readSocketMessage(t, reconnected)
	if result.Type != MessageRunResult {
		t.Fatalf("expected %q, got %q", MessageRunResult, result.Type)
	}
	if result.RequestID != requestID {
		t.Fatalf("expected request %q, got %q", requestID, result.RequestID)
	}
	if result.Result != raw {
		t.Fatalf("expected last run result to be replayed")
	}
}

func TestMCPActivityExpires(t *testing.T) {
	previousWindow := mcpIdleWindow
	mcpIdleWindow = 25 * time.Millisecond
	t.Cleanup(func() { mcpIdleWindow = previousWindow })

	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	mcpSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/session/" + sessionID + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() { _ = mcpSession.Close() })

	active := readSocketMessage(t, conn)
	if active.Type != MessageMCPStatus || active.MCPStatus != MCPStatusActive {
		t.Fatalf("expected active MCP status, got %#v", active)
	}

	idle := readSocketMessage(t, conn)
	if idle.Type != MessageMCPStatus || idle.MCPStatus != MCPStatusIdle {
		t.Fatalf("expected idle MCP status, got %#v", idle)
	}
	if idle.MCPLastSeen == "" {
		t.Fatal("expected last-seen timestamp")
	}
}

func TestMCPExecuteFlow(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	mcpSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/session/" + sessionID + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() { _ = mcpSession.Close() })

	status := readSocketMessage(t, conn)
	if status.Type != MessageMCPStatus || status.MCPStatus != MCPStatusActive {
		t.Fatalf("expected active MCP status, got %#v", status)
	}

	type callResult struct {
		result *mcp.CallToolResult
		err    error
	}
	callDone := make(chan callResult, 1)
	go func() {
		result, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"code": "1 + 1"},
		})
		callDone <- callResult{result: result, err: err}
	}()

	execute := readSocketMessage(t, conn)
	if execute.Type != MessageExecuteRequest {
		t.Fatalf("expected %q, got %q", MessageExecuteRequest, execute.Type)
	}
	if execute.RequestID == "" || execute.Code != "1 + 1" {
		t.Fatalf("unexpected execute request: %#v", execute)
	}

	writeSocketMessage(t, conn, socketMessage{
		Type:      MessageRunSubmit,
		RequestID: execute.RequestID,
		Code:      execute.Code,
	})

	accepted := readSocketMessage(t, conn)
	if accepted.Type != MessageRunAccepted {
		t.Fatalf("expected %q, got %q (%s)", MessageRunAccepted, accepted.Type, accepted.Error)
	}

	jobID, err := uuid.Parse(accepted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}

	raw := encodedResult("2")
	manager.DeliverJobResult(jobID, "Success", raw)

	result := readSocketMessage(t, conn)
	if result.Type != MessageRunResult || result.RequestID != execute.RequestID {
		t.Fatalf("unexpected run result: %#v", result)
	}

	select {
	case call := <-callDone:
		if call.err != nil {
			t.Fatalf("CallTool failed: %v", call.err)
		}
		content, err := json.Marshal(call.result.Content)
		if err != nil {
			t.Fatalf("marshal tool content: %v", err)
		}
		var contents []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(content, &contents); err != nil {
			t.Fatalf("decode tool content: %v", err)
		}
		if len(contents) == 0 || !strings.Contains(contents[0].Text, `"output":"2"`) {
			t.Fatalf("expected tool output 2, got %s", content)
		}
		if !strings.Contains(contents[0].Text, jobID.String()) {
			t.Fatalf("expected job id in tool output, got %s", content)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for tool result")
	}
}

func TestMCPExecuteBroadcastsToEveryTab(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	session, err := manager.EnsureSession(uuid.NewString())
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	first, _ := dialSessionByID(t, server, session.SessionID)
	second, _ := dialSessionByID(t, server, session.SessionID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	mcpSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/session/" + session.SessionID + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() { _ = mcpSession.Close() })

	firstStatus := readSocketMessage(t, first)
	secondStatus := readSocketMessage(t, second)
	if firstStatus.Type != MessageMCPStatus || secondStatus.Type != MessageMCPStatus {
		t.Fatalf("expected MCP status on both tabs, got %q and %q", firstStatus.Type, secondStatus.Type)
	}

	type callResult struct {
		result *mcp.CallToolResult
		err    error
	}
	callDone := make(chan callResult, 1)
	go func() {
		result, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"code": "2 + 2"},
		})
		callDone <- callResult{result: result, err: err}
	}()

	firstExecute := readSocketMessage(t, first)
	secondExecute := readSocketMessage(t, second)
	if firstExecute.Type != MessageExecuteRequest || secondExecute.Type != MessageExecuteRequest {
		t.Fatalf("expected both tabs to receive %q, got %q and %q", MessageExecuteRequest, firstExecute.Type, secondExecute.Type)
	}
	if firstExecute.RequestID != secondExecute.RequestID || firstExecute.Code != "2 + 2" {
		t.Fatalf("unexpected execute requests: %#v and %#v", firstExecute, secondExecute)
	}

	for _, conn := range []*websocket.Conn{first, second} {
		writeSocketMessage(t, conn, socketMessage{
			Type:      MessageRunSubmit,
			RequestID: firstExecute.RequestID,
			Code:      firstExecute.Code,
		})
	}

	firstAccepted := readSocketMessage(t, first)
	secondAccepted := readSocketMessage(t, second)
	if firstAccepted.Type != MessageRunAccepted || secondAccepted.Type != MessageRunAccepted {
		t.Fatalf("expected both tabs to receive %q, got %q and %q", MessageRunAccepted, firstAccepted.Type, secondAccepted.Type)
	}
	if firstAccepted.JobID != secondAccepted.JobID {
		t.Fatalf("expected a shared job, got %q and %q", firstAccepted.JobID, secondAccepted.JobID)
	}

	jobID, err := uuid.Parse(firstAccepted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	manager.DeliverJobResult(jobID, "Success", encodedResult("4"))

	firstResult := readSocketMessage(t, first)
	secondResult := readSocketMessage(t, second)
	if firstResult.Type != MessageRunResult || secondResult.Type != MessageRunResult {
		t.Fatalf("expected both tabs to receive %q, got %q and %q", MessageRunResult, firstResult.Type, secondResult.Type)
	}

	select {
	case call := <-callDone:
		if call.err != nil {
			t.Fatalf("CallTool failed: %v", call.err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for tool result")
	}
}

func TestMCPExecuteRejected(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	mcpSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/session/" + sessionID + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() { _ = mcpSession.Close() })

	status := readSocketMessage(t, conn)
	if status.Type != MessageMCPStatus || status.MCPStatus != MCPStatusActive {
		t.Fatalf("expected active MCP status, got %#v", status)
	}

	type callResult struct {
		result *mcp.CallToolResult
		err    error
	}
	callDone := make(chan callResult, 1)
	go func() {
		result, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"code": "1 + 1"},
		})
		callDone <- callResult{result: result, err: err}
	}()

	execute := readSocketMessage(t, conn)
	if execute.Type != MessageExecuteRequest {
		t.Fatalf("expected %q, got %q", MessageExecuteRequest, execute.Type)
	}

	writeSocketMessage(t, conn, socketMessage{
		Type:      MessageExecuteRejected,
		RequestID: execute.RequestID,
		Error:     "editor is busy",
	})

	select {
	case call := <-callDone:
		if call.err != nil {
			if !strings.Contains(call.err.Error(), "editor is busy") {
				t.Fatalf("expected rejection error, got %v", call.err)
			}
			return
		}
		content, err := json.Marshal(call.result.Content)
		if err != nil {
			t.Fatalf("marshal tool content: %v", err)
		}
		if !call.result.IsError {
			t.Fatalf("expected IsError, got %s", content)
		}
		if !strings.Contains(string(content), "editor is busy") {
			t.Fatalf("expected rejection content, got %s", content)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for rejected tool result")
	}
}
