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

func (f *fakeStore) InsertNewJob(src string, _ context.Context) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job := db.UserScriptJob{
		JobId:     uuid.New(),
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

func newTestServer(t *testing.T, manager *Manager) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", manager.HandleCreateSession)
	mux.HandleFunc("GET /ws/{session_id}", manager.HandleWS)
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
		"output":     output,
		"debug_info": map[string]any{},
		"ast":        map[string]any{},
	})
	return base64.StdEncoding.EncodeToString(payload)
}

func TestWebSocketRequiresExistingSession(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/" + uuid.NewString()
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected unknown session websocket to be rejected")
	}
	if response == nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %#v", response)
	}
}

func TestWebSocketRunFlow(t *testing.T) {
	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)

	server := newTestServer(t, manager)
	conn, _ := dialSession(t, server, manager)

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
