package mcpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"go_js_demo/db"
	"go_js_demo/worker"
	"io"
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
	mu            sync.Mutex
	jobs          map[uuid.UUID]*db.UserScriptJob
	metadataReads int
	resultReads   int
}

func newFakeStore() *fakeStore {
	return &fakeStore{jobs: make(map[uuid.UUID]*db.UserScriptJob)}
}

func metadataCopy(job *db.UserScriptJob) *db.UserScriptJob {
	copy := *job
	copy.Result = ""
	return &copy
}

func (f *fakeStore) InsertNewJob(sessionID, clientRequestID, src string, _ context.Context) (*db.UserScriptJob, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if clientRequestID != "" {
		for _, existing := range f.jobs {
			if existing.SessionID == sessionID && existing.ClientRequestID == clientRequestID {
				if existing.Src != src {
					return nil, false, db.ErrClientRequestConflict
				}
				return metadataCopy(existing), false, nil
			}
		}
	}

	now := time.Now()
	job := &db.UserScriptJob{
		JobID:           uuid.New(),
		SessionID:       sessionID,
		Src:             src,
		JobStatus:       db.JobPending,
		ResultEncoding:  "base64",
		ClientRequestID: clientRequestID,
		Revision:        1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	f.jobs[job.JobID] = job
	return metadataCopy(job), true, nil
}

func (f *fakeStore) UpdateJob(jobID uuid.UUID, completion db.Completion, _ context.Context) (*db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, ok := f.jobs[jobID]
	if !ok {
		return nil, db.ErrJobNotFound
	}
	now := time.Now()
	job.JobStatus = completion.Status
	job.Result = completion.Result
	job.ResultEncoding = completion.ResultEncoding
	job.ResultFormatVersion = completion.ResultFormatVersion
	job.Output = completion.Output
	job.Revision++
	job.UpdatedAt = now
	job.CompletedAt = &now
	return metadataCopy(job), nil
}

func (f *fakeStore) GetJobsBySessionID(sessionID string, _ context.Context) ([]db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metadataReads++

	jobs := make([]db.UserScriptJob, 0)
	for _, job := range f.jobs {
		if job.SessionID == sessionID {
			jobs = append(jobs, *metadataCopy(job))
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
	f.metadataReads++

	for _, job := range f.jobs {
		if job.JobStatus == db.JobFailed {
			continue
		}
		return metadataCopy(job), nil
	}
	return nil, db.ErrNoJobs
}

func (f *fakeStore) GetJobBySessionAndID(sessionID string, jobID uuid.UUID, _ context.Context) (*db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metadataReads++

	job, ok := f.jobs[jobID]
	if !ok || job.SessionID != sessionID {
		return nil, db.ErrJobNotFound
	}
	return metadataCopy(job), nil
}

func (f *fakeStore) GetJobBySessionAndClientRequestID(sessionID, clientRequestID string, _ context.Context) (*db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metadataReads++

	for _, job := range f.jobs {
		if job.SessionID == sessionID && job.ClientRequestID == clientRequestID {
			return metadataCopy(job), nil
		}
	}
	return nil, db.ErrJobNotFound
}

func (f *fakeStore) GetJobResultBySessionAndID(sessionID string, jobID uuid.UUID, _ context.Context) (*db.UserScriptJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resultReads++

	job, ok := f.jobs[jobID]
	if !ok || job.SessionID != sessionID {
		return nil, db.ErrJobNotFound
	}
	copy := *job
	return &copy, nil
}

func (f *fakeStore) storedJob(jobID uuid.UUID) (*db.UserScriptJob, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.jobs[jobID]
	if !ok {
		return nil, false
	}
	copy := *job
	return &copy, true
}

func (f *fakeStore) jobCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.jobs)
}

func newTestServer(t *testing.T, manager *Manager) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", manager.HandleCreateSession)
	mux.HandleFunc("GET /ws/{session_id}", manager.HandleWS)
	mux.HandleFunc("GET /session/{session_id}/jobs", manager.HandleSessionJobs)
	mux.HandleFunc("POST /session/{session_id}/jobs", manager.HandleCreateJob)
	mux.HandleFunc("GET /session/{session_id}/jobs/{job_id}", manager.HandleJobStatus)
	mux.HandleFunc("GET /session/{session_id}/jobs/{job_id}/result", manager.HandleJobResult)
	mux.HandleFunc("GET /scripts/random", manager.HandleRandomJob)
	mux.HandleFunc("/session/{session_id}/mcp", manager.HandleMCP)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newTestManager(t *testing.T) (*fakeStore, chan worker.HeartbeatSignal, *Manager) {
	t.Helper()

	store := newFakeStore()
	work := make(chan worker.HeartbeatSignal, 8)
	manager := NewManager(store, work, time.Minute)
	t.Cleanup(manager.Close)
	return store, work, manager
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
		"debug_chunks":   []any{},
		"ast":            map[string]any{},
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
				"max_work_items":  16,
				"max_value_chars": 160,
			},
		},
	})
	return base64.StdEncoding.EncodeToString(payload)
}

func submitJob(t *testing.T, server *httptest.Server, sessionID, clientRequestID, executeRequestID, code string) (int, http.Header, createJobResponse) {
	t.Helper()

	body, err := json.Marshal(createJobRequest{
		ClientRequestID:  clientRequestID,
		ExecuteRequestID: executeRequestID,
		Code:             code,
	})
	if err != nil {
		t.Fatalf("marshal job request: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+"/session/"+sessionID+"/jobs", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create job request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("submit job: %v", err)
	}
	defer response.Body.Close()

	var result createJobResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode job response: %v", err)
	}
	return response.StatusCode, response.Header.Clone(), result
}

func completeJob(t *testing.T, store *fakeStore, manager *Manager, jobID uuid.UUID, completion db.Completion) db.UserScriptJob {
	t.Helper()

	updated, err := store.UpdateJob(jobID, completion, context.Background())
	if err != nil {
		t.Fatalf("update job: %v", err)
	}

	update := worker.JobResult{
		SessionID:       updated.SessionID,
		ClientRequestID: updated.ClientRequestID,
		JobID:           updated.JobID,
		Status:          updated.JobStatus,
		Revision:        updated.Revision,
	}
	if updated.JobStatus == db.JobSucceeded {
		update.Output = completion.Output
	} else {
		update.ErrorMessage = completion.Result
	}
	manager.DeliverJobUpdate(update)
	return *updated
}

func markProcessing(t *testing.T, store *fakeStore, manager *Manager, jobID uuid.UUID) db.UserScriptJob {
	t.Helper()

	store.mu.Lock()
	job, ok := store.jobs[jobID]
	if !ok {
		store.mu.Unlock()
		t.Fatalf("job %s not found", jobID)
	}
	job.JobStatus = db.JobProcessing
	job.Revision++
	job.UpdatedAt = time.Now()
	processing := *job
	store.mu.Unlock()

	manager.DeliverJobUpdate(worker.JobResult{
		SessionID:       processing.SessionID,
		ClientRequestID: processing.ClientRequestID,
		JobID:           processing.JobID,
		Status:          processing.JobStatus,
		Revision:        processing.Revision,
	})
	return processing
}

func TestWebSocketCreatesSessionForClientID(t *testing.T) {
	_, _, manager := newTestManager(t)
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
	_, _, manager := newTestManager(t)
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
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	clientID := uuid.NewString()
	otherClientID := uuid.NewString()
	ctx := context.Background()

	first, _, err := store.InsertNewJob(clientID, uuid.NewString(), "console.log(1)", ctx)
	if err != nil {
		t.Fatalf("insert first job: %v", err)
	}
	if _, _, err := store.InsertNewJob(clientID, uuid.NewString(), "console.log(2)", ctx); err != nil {
		t.Fatalf("insert second job: %v", err)
	}
	if _, _, err := store.InsertNewJob(otherClientID, uuid.NewString(), "console.log(3)", ctx); err != nil {
		t.Fatalf("insert other job: %v", err)
	}
	if _, err := store.UpdateJob(first.JobID, db.Completion{
		Status:         db.JobSucceeded,
		Result:         encodedResult("1"),
		ResultEncoding: "base64",
		Output:         "1",
	}, ctx); err != nil {
		t.Fatalf("complete first job: %v", err)
	}

	response, err := http.Get(server.URL + "/session/" + clientID + "/jobs")
	if err != nil {
		t.Fatalf("list scripts: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read scripts: %v", err)
	}
	if strings.Contains(string(body), `"result"`) {
		t.Fatalf("history response exposed the result column: %s", body)
	}

	var jobs []scriptSummary
	if err := json.Unmarshal(body, &jobs); err != nil {
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
	if store.resultReads != 0 {
		t.Fatalf("history read the result column %d times", store.resultReads)
	}
}

func TestSessionJobsRejectsInvalidSessionID(t *testing.T) {
	_, _, manager := newTestManager(t)
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
	store, _, manager := newTestManager(t)
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

	failedJob, _, err := store.InsertNewJob(uuid.NewString(), uuid.NewString(), "console.log('failed')", ctx)
	if err != nil {
		t.Fatalf("insert failed job: %v", err)
	}
	if _, err := store.UpdateJob(failedJob.JobID, db.Completion{
		Status:         db.JobFailed,
		Result:         "nope",
		ResultEncoding: "text",
		Output:         "nope",
	}, ctx); err != nil {
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
	if _, _, err := store.InsertNewJob(sessionID, uuid.NewString(), "console.log('random')", ctx); err != nil {
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
	if store.resultReads != 0 {
		t.Fatalf("random job read the result column %d times", store.resultReads)
	}
}

func TestReconnectReceivesLastJobMetadata(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	clientRequestID := uuid.NewString()
	status, _, created := submitJob(t, server, sessionID, clientRequestID, "", "console.log(3)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	pending := readSocketMessage(t, conn)
	if pending.Type != MessageRunStatus || pending.JobStatus != "Pending" {
		t.Fatalf("expected pending status, got %#v", pending)
	}
	jobID, err := uuid.Parse(created.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}

	completeJob(t, store, manager, jobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("3"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "3",
	})
	terminal := readSocketMessage(t, conn)
	if terminal.Type != MessageRunStatus || terminal.JobStatus != "Success" {
		t.Fatalf("expected success status, got %#v", terminal)
	}

	_ = conn.Close()
	time.Sleep(20 * time.Millisecond)

	reconnected, ready := dialSessionByID(t, server, sessionID)
	if ready.Type != MessageSessionReady {
		t.Fatalf("expected %q, got %q", MessageSessionReady, ready.Type)
	}
	if ready.ActiveRun != nil {
		t.Fatalf("expected no active run, got %#v", ready.ActiveRun)
	}
	if ready.LastJob == nil {
		t.Fatal("expected last job metadata")
	}
	if ready.LastJob.JobID != jobID.String() || ready.LastJob.JobStatus != "Success" {
		t.Fatalf("unexpected last job: %#v", ready.LastJob)
	}
	if ready.LastJob.ClientRequestID != clientRequestID {
		t.Fatalf("expected client request %q, got %q", clientRequestID, ready.LastJob.ClientRequestID)
	}
	if ready.LastJob.ResultURL == "" {
		t.Fatal("expected a result URL for the last job")
	}
	_ = reconnected
}

func TestMCPActivityExpires(t *testing.T) {
	previousWindow := mcpIdleWindow
	mcpIdleWindow = 25 * time.Millisecond
	t.Cleanup(func() { mcpIdleWindow = previousWindow })

	_, _, manager := newTestManager(t)
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
	store, _, manager := newTestManager(t)
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

	httpStatus, _, submitted := submitJob(t, server, sessionID, execute.RequestID, execute.RequestID, execute.Code)
	if httpStatus != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", httpStatus)
	}
	jobID, err := uuid.Parse(submitted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	pending := readSocketMessage(t, conn)
	if pending.Type != MessageRunStatus || pending.JobStatus != "Pending" {
		t.Fatalf("expected pending status, got %#v", pending)
	}

	completeJob(t, store, manager, jobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("2"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "2",
	})
	terminal := readSocketMessage(t, conn)
	if terminal.Type != MessageRunStatus || terminal.JobStatus != "Success" {
		t.Fatalf("unexpected run status: %#v", terminal)
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
	store, _, manager := newTestManager(t)
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

	firstHTTPStatus, _, firstSubmitted := submitJob(t, server, session.SessionID, firstExecute.RequestID, firstExecute.RequestID, firstExecute.Code)
	secondHTTPStatus, _, secondSubmitted := submitJob(t, server, session.SessionID, secondExecute.RequestID, secondExecute.RequestID, secondExecute.Code)
	if firstHTTPStatus != http.StatusAccepted || secondHTTPStatus != http.StatusAccepted {
		t.Fatalf("expected both submissions to succeed, got %d and %d", firstHTTPStatus, secondHTTPStatus)
	}
	if firstSubmitted.JobID != secondSubmitted.JobID {
		t.Fatalf("expected one job, got %q and %q", firstSubmitted.JobID, secondSubmitted.JobID)
	}
	if store.jobCount() != 1 {
		t.Fatalf("expected one stored job, got %d", store.jobCount())
	}

	jobID, err := uuid.Parse(firstSubmitted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	completeJob(t, store, manager, jobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("4"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "4",
	})

	select {
	case call := <-callDone:
		if call.err != nil {
			t.Fatalf("CallTool failed: %v", call.err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for tool result")
	}
}

func TestMCPExecuteKeepsPendingWhenOnlyOneTabRejects(t *testing.T) {
	store, _, manager := newTestManager(t)
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

	_ = readSocketMessage(t, first)
	_ = readSocketMessage(t, second)

	type callResult struct {
		result *mcp.CallToolResult
		err    error
	}
	callDone := make(chan callResult, 1)
	go func() {
		result, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"code": "4 + 4"},
		})
		callDone <- callResult{result: result, err: err}
	}()

	firstExecute := readSocketMessage(t, first)
	secondExecute := readSocketMessage(t, second)
	if firstExecute.Type != MessageExecuteRequest || secondExecute.Type != MessageExecuteRequest {
		t.Fatalf("expected both tabs to receive the execute request, got %q and %q", firstExecute.Type, secondExecute.Type)
	}

	writeSocketMessage(t, first, socketMessage{
		Type:      MessageExecuteRejected,
		RequestID: firstExecute.RequestID,
		Error:     "the editor is busy",
	})
	time.Sleep(50 * time.Millisecond)

	status, _, submitted := submitJob(t, server, session.SessionID, secondExecute.RequestID, secondExecute.RequestID, secondExecute.Code)
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	jobID, err := uuid.Parse(submitted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	_ = readSocketMessage(t, first)
	_ = readSocketMessage(t, second)

	completeJob(t, store, manager, jobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("8"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "8",
	})

	select {
	case call := <-callDone:
		if call.err != nil {
			t.Fatalf("CallTool failed after a single tab rejected: %v", call.err)
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
		if len(contents) == 0 || !strings.Contains(contents[0].Text, `"output":"8"`) {
			t.Fatalf("expected tool output 8, got %s", content)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for the second tab to execute the request")
	}
}

func TestMCPExecuteRejected(t *testing.T) {
	_, _, manager := newTestManager(t)
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

func TestWorkerUpdateBeforeSessionBind(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, err := manager.EnsureSession(uuid.NewString())
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	job, _, err := store.InsertNewJob(session.SessionID, uuid.NewString(), "console.log(5)", context.Background())
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	processing := markProcessing(t, store, manager, job.JobID)

	conn, ready := dialSessionByID(t, server, session.SessionID)
	if ready.ActiveRun == nil {
		t.Fatal("expected active run metadata")
	}
	if ready.ActiveRun.JobID != job.JobID.String() || ready.ActiveRun.JobStatus != "Processing" {
		t.Fatalf("unexpected active run: %#v", ready.ActiveRun)
	}
	if ready.ActiveRun.Version != processing.Revision {
		t.Fatalf("expected version %d, got %d", processing.Revision, ready.ActiveRun.Version)
	}

	completeJob(t, store, manager, job.JobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("5"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "5",
	})
	terminal := readSocketMessage(t, conn)
	if terminal.Type != MessageRunStatus || terminal.JobStatus != "Success" {
		t.Fatalf("unexpected terminal update: %#v", terminal)
	}
}

func TestTerminalUpdateBeforeSubmitReturns(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, err := manager.EnsureSession(uuid.NewString())
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	clientRequestID := uuid.NewString()
	job, _, err := store.InsertNewJob(session.SessionID, clientRequestID, "console.log(6)", context.Background())
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if _, err := store.UpdateJob(job.JobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("6"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "6",
	}, context.Background()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	status, _, submitted := submitJob(t, server, session.SessionID, clientRequestID, "", "console.log(6)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	if submitted.JobID != job.JobID.String() {
		t.Fatalf("expected job %q, got %q", job.JobID, submitted.JobID)
	}
	if submitted.JobStatus != "Success" {
		t.Fatalf("expected terminal status, got %q", submitted.JobStatus)
	}
	if submitted.Created {
		t.Fatal("expected an idempotent submission")
	}
	if submitted.ResultURL == nil || *submitted.ResultURL == "" {
		t.Fatal("expected a result URL")
	}
}

func TestMCPExecuteOutputComesFromWorker(t *testing.T) {
	store, _, manager := newTestManager(t)
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

	_ = readSocketMessage(t, conn)

	type callResult struct {
		result *mcp.CallToolResult
		err    error
	}
	callDone := make(chan callResult, 1)
	go func() {
		result, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute",
			Arguments: map[string]any{"code": "3 + 3"},
		})
		callDone <- callResult{result: result, err: err}
	}()

	execute := readSocketMessage(t, conn)
	status, _, submitted := submitJob(t, server, sessionID, execute.RequestID, execute.RequestID, execute.Code)
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	jobID, err := uuid.Parse(submitted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	_ = readSocketMessage(t, conn)

	store.mu.Lock()
	job := store.jobs[jobID]
	job.JobStatus = db.JobSucceeded
	job.Result = "not-base64"
	job.ResultEncoding = "text"
	job.Output = "6"
	job.Revision++
	job.UpdatedAt = time.Now()
	revision := job.Revision
	clientRequestID := job.ClientRequestID
	store.mu.Unlock()

	manager.DeliverJobUpdate(worker.JobResult{
		SessionID:       sessionID,
		ClientRequestID: clientRequestID,
		JobID:           jobID,
		Status:          db.JobSucceeded,
		Revision:        revision,
		Output:          "6",
	})

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
		if len(contents) == 0 || !strings.Contains(contents[0].Text, `"output":"6"`) {
			t.Fatalf("expected worker output, got %s", content)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for tool result")
	}
}

func TestResultEndpointReadsResultColumn(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, err := manager.EnsureSession(uuid.NewString())
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	job, _, err := store.InsertNewJob(session.SessionID, uuid.NewString(), "console.log(1)", context.Background())
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if _, err := store.UpdateJob(job.JobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("1"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "1",
	}, context.Background()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	response, err := http.Get(server.URL + "/session/" + session.SessionID + "/jobs/" + job.JobID.String() + "/result")
	if err != nil {
		t.Fatalf("get result: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if store.resultReads != 1 {
		t.Fatalf("expected one result read, got %d", store.resultReads)
	}
}

func TestJobStatusDoesNotReadResultColumn(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, err := manager.EnsureSession(uuid.NewString())
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	job, _, err := store.InsertNewJob(session.SessionID, uuid.NewString(), "console.log(1)", context.Background())
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if _, err := store.UpdateJob(job.JobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("1"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "1",
	}, context.Background()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	response, err := http.Get(server.URL + "/session/" + session.SessionID + "/jobs/" + job.JobID.String())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if store.resultReads != 0 {
		t.Fatalf("status read the result column %d times", store.resultReads)
	}
}
