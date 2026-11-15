package mcpserver

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"go_js_demo/db"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func resultRequest(t *testing.T, server interface{ Client() *http.Client }, rawURL, acceptEncoding, ifNoneMatch string) *http.Response {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("create result request: %v", err)
	}
	if acceptEncoding != "" {
		request.Header.Set("Accept-Encoding", acceptEncoding)
	}
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("get result: %v", err)
	}
	return response
}

func insertPendingJob(t *testing.T, store *fakeStore, sessionID, src string) *db.UserScriptJob {
	t.Helper()

	job, _, err := store.InsertNewJob(sessionID, uuid.NewString(), src, context.Background())
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	return job
}

func completeStoredJob(t *testing.T, store *fakeStore, jobID uuid.UUID, completion db.Completion) {
	t.Helper()

	if _, err := store.UpdateJob(jobID, completion, context.Background()); err != nil {
		t.Fatalf("complete job: %v", err)
	}
}

func insertSuccessJob(t *testing.T, store *fakeStore, sessionID, output string) *db.UserScriptJob {
	t.Helper()

	job := insertPendingJob(t, store, sessionID, "console.log("+output+")")
	completeStoredJob(t, store, job.JobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult(output),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              output,
	})
	return job
}

func insertFailedJob(t *testing.T, store *fakeStore, sessionID, message string) *db.UserScriptJob {
	t.Helper()

	job := insertPendingJob(t, store, sessionID, "throw new Error('nope')")
	completeStoredJob(t, store, job.JobID, db.Completion{
		Status:         db.JobFailed,
		Result:         message,
		ResultEncoding: "text",
		Output:         message,
	})
	return job
}

func TestCreateJobReturnsAcceptedPending(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	clientRequestID := uuid.NewString()

	status, headers, response := submitJob(t, server, session.SessionID, clientRequestID, "", "console.log(1)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	if headers.Get("Location") != response.StatusURL {
		t.Fatalf("expected location %q, got %q", response.StatusURL, headers.Get("Location"))
	}
	if response.JobStatus != "Pending" || response.Version != 1 || !response.Created {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.ClientRequestID != clientRequestID {
		t.Fatalf("expected client request %q, got %q", clientRequestID, response.ClientRequestID)
	}
	if response.ResultURL != nil {
		t.Fatalf("expected no result URL, got %q", *response.ResultURL)
	}
	if response.StatusURL != "/session/"+session.SessionID+"/jobs/"+response.JobID {
		t.Fatalf("unexpected status URL: %q", response.StatusURL)
	}
}

func TestCreateJobIsIdempotent(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	clientRequestID := uuid.NewString()

	firstStatus, _, first := submitJob(t, server, session.SessionID, clientRequestID, "", "console.log(1)")
	secondStatus, _, second := submitJob(t, server, session.SessionID, clientRequestID, "", "console.log(1)")
	if firstStatus != http.StatusAccepted || secondStatus != http.StatusAccepted {
		t.Fatalf("expected both requests to succeed, got %d and %d", firstStatus, secondStatus)
	}
	if first.JobID != second.JobID {
		t.Fatalf("expected the same job, got %q and %q", first.JobID, second.JobID)
	}
	if first.Created != true || second.Created != false {
		t.Fatalf("unexpected created flags: %v and %v", first.Created, second.Created)
	}
	if store.jobCount() != 1 {
		t.Fatalf("expected one job, got %d", store.jobCount())
	}
}

func TestCreateJobSameKeyDifferentSourceConflicts(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	clientRequestID := uuid.NewString()

	status, _, _ := submitJob(t, server, session.SessionID, clientRequestID, "", "console.log(1)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}

	status, _, response := submitJob(t, server, session.SessionID, clientRequestID, "", "console.log(2)")
	if status != http.StatusConflict {
		t.Fatalf("expected 409, got %d", status)
	}
	if response.Error == nil || response.Error.Code != "idempotency_conflict" {
		t.Fatalf("unexpected error: %#v", response.Error)
	}
}

func TestCreateJobRejectsBusySession(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())

	status, _, _ := submitJob(t, server, session.SessionID, uuid.NewString(), "", "console.log(1)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}

	status, _, response := submitJob(t, server, session.SessionID, uuid.NewString(), "", "console.log(2)")
	if status != http.StatusConflict {
		t.Fatalf("expected 409, got %d", status)
	}
	if response.Error == nil || response.Error.Code != "session_busy" {
		t.Fatalf("unexpected error: %#v", response.Error)
	}
}

func TestCreateJobRejectsOversizedCode(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())

	status, _, response := submitJob(t, server, session.SessionID, uuid.NewString(), "", strings.Repeat("a", maxCodeSize+1))
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", status)
	}
	if response.Error == nil || response.Error.Code != "code_too_large" {
		t.Fatalf("unexpected error: %#v", response.Error)
	}
}

func TestCreateJobRequiresClientRequestID(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())

	status, _, response := submitJob(t, server, session.SessionID, "", "", "console.log(1)")
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", status)
	}
	if response.Error == nil || response.Error.Code != "invalid_request" {
		t.Fatalf("unexpected error: %#v", response.Error)
	}
}

func TestJobStatusPending(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertPendingJob(t, store, session.SessionID, "console.log(1)")

	response, err := http.Get(server.URL + "/session/" + session.SessionID + "/jobs/" + job.JobID.String())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	var status jobStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.JobStatus != "Pending" || status.Version != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.ResultURL != "" {
		t.Fatalf("expected no result URL, got %q", status.ResultURL)
	}
}

func TestJobStatusSuccessHasResultURL(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "ok")

	response, err := http.Get(server.URL + "/session/" + session.SessionID + "/jobs/" + job.JobID.String())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer response.Body.Close()

	var status jobStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.JobStatus != "Success" || status.ResultURL == "" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.Error != nil {
		t.Fatalf("unexpected error: %#v", status.Error)
	}
}

func TestJobStatusFailedHasError(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertFailedJob(t, store, session.SessionID, "boom")

	response, err := http.Get(server.URL + "/session/" + session.SessionID + "/jobs/" + job.JobID.String())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer response.Body.Close()

	var status jobStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.JobStatus != "Failed" || status.ResultURL != "" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.Error == nil || status.Error.Code != "execution_failed" || status.Error.Message != "boom" {
		t.Fatalf("unexpected error: %#v", status.Error)
	}
}

func TestJobStatusIsSessionScoped(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	other, _ := manager.EnsureSession(uuid.NewString())
	job := insertPendingJob(t, store, session.SessionID, "console.log(1)")

	response, err := http.Get(server.URL + "/session/" + other.SessionID + "/jobs/" + job.JobID.String())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.StatusCode)
	}
}

func TestJobStatusUnknownReturns404(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())

	response, err := http.Get(server.URL + "/session/" + session.SessionID + "/jobs/" + uuid.NewString())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.StatusCode)
	}
}

func TestJobResultReturnsRawJSON(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "hello")

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if response.Header.Get("Content-Encoding") != "" {
		t.Fatalf("unexpected content encoding %q", response.Header.Get("Content-Encoding"))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if len(body) == 0 || body[0] != '{' {
		t.Fatalf("expected a raw JSON object, got %q", body)
	}

	var decoded struct {
		FormatVersion int    `json:"format_version"`
		Output        string `json:"output"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded.FormatVersion != 2 || decoded.Output != "hello" {
		t.Fatalf("unexpected result: %#v", decoded)
	}
}

func TestJobResultGzipWhenAccepted(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "gzip")

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "gzip", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if response.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip, got %q", response.Header.Get("Content-Encoding"))
	}

	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("create gzip reader: %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip result: %v", err)
	}
	var decoded struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded.Output != "gzip" {
		t.Fatalf("unexpected output %q", decoded.Output)
	}
}

func TestJobResultCompressesLargePayloadWithoutAcceptEncoding(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	largeOutput := strings.Repeat("large-result-", largeResultCompressionThreshold/len("large-result-")+1)
	job := insertSuccessJob(t, store, session.SessionID, largeOutput)

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if response.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip for a large payload, got %q", response.Header.Get("Content-Encoding"))
	}

	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("create gzip reader: %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip result: %v", err)
	}
	var decoded struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded.Output != largeOutput {
		t.Fatalf("unexpected output length %d, expected %d", len(decoded.Output), len(largeOutput))
	}
}

func TestJobResultCompressesLargePayloadWhenProxyRequestsIdentity(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	largeOutput := strings.Repeat("large-result-", largeResultCompressionThreshold/len("large-result-")+1)
	job := insertSuccessJob(t, store, session.SessionID, largeOutput)

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if response.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip for a large payload, got %q", response.Header.Get("Content-Encoding"))
	}
}

func TestJobResultLargePayloadHonorsGzipQZero(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	largeOutput := strings.Repeat("large-result-", largeResultCompressionThreshold/len("large-result-")+1)
	job := insertSuccessJob(t, store, session.SessionID, largeOutput)

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "gzip;q=0, *;q=1", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if response.Header.Get("Content-Encoding") != "" {
		t.Fatalf("expected identity when gzip is explicitly refused, got %q", response.Header.Get("Content-Encoding"))
	}
}

func TestJobResultIdentityWhenGzipNotAccepted(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "plain")

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "", "")
	defer response.Body.Close()
	if response.Header.Get("Content-Encoding") != "" {
		t.Fatalf("expected identity, got %q", response.Header.Get("Content-Encoding"))
	}
}

func TestJobResultGzipQZero(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "plain")

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "gzip;q=0, *;q=1", "")
	defer response.Body.Close()
	if response.Header.Get("Content-Encoding") != "" {
		t.Fatalf("expected identity, got %q", response.Header.Get("Content-Encoding"))
	}
}

func TestJobResultLegacyBase64(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertPendingJob(t, store, session.SessionID, "console.log(1)")
	completeStoredJob(t, store, job.JobID, db.Completion{
		Status:         db.JobSucceeded,
		Result:         encodedResult("legacy"),
		ResultEncoding: "",
		Output:         "legacy",
	})

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var decoded struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded.Output != "legacy" {
		t.Fatalf("unexpected output %q", decoded.Output)
	}
}

func TestJobResultPendingReturns202(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertPendingJob(t, store, session.SessionID, "console.log(1)")

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", response.StatusCode)
	}
	if response.Header.Get("Retry-After") != "1" {
		t.Fatalf("expected retry-after 1, got %q", response.Header.Get("Retry-After"))
	}
	if response.Header.Get("Location") == "" {
		t.Fatal("expected a status location")
	}
}

func TestJobResultFailedReturns409(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertFailedJob(t, store, session.SessionID, "boom")

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.StatusCode)
	}
	var failure struct {
		Error apiError `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
		t.Fatalf("decode failure: %v", err)
	}
	if failure.Error.Code != "execution_failed" || failure.Error.Message != "boom" {
		t.Fatalf("unexpected failure: %#v", failure.Error)
	}
}

func TestJobResultIsSessionScoped(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	other, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "scoped")

	response := resultRequest(t, server, server.URL+"/session/"+other.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.StatusCode)
	}
}

func TestJobResultUnicodeRoundTrip(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	output := "héllo 🚀 → ✓"
	job := insertSuccessJob(t, store, session.SessionID, output)

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/"+job.JobID.String()+"/result", "identity", "")
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var decoded struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded.Output != output {
		t.Fatalf("expected %q, got %q", output, decoded.Output)
	}
}

func TestJobResultETag304(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	job := insertSuccessJob(t, store, session.SessionID, "cached")
	resultURL := server.URL + "/session/" + session.SessionID + "/jobs/" + job.JobID.String() + "/result"

	first := resultRequest(t, server, resultURL, "gzip", "")
	first.Body.Close()
	etag := first.Header.Get("ETag")
	if etag == "" {
		t.Fatal("expected an ETag")
	}

	second := resultRequest(t, server, resultURL, "gzip", etag)
	defer second.Body.Close()
	if second.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", second.StatusCode)
	}
	if second.Header.Get("ETag") != etag {
		t.Fatalf("expected ETag %q, got %q", etag, second.Header.Get("ETag"))
	}
}

func TestMultipleTabsReceiveJobStatus(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())
	first, _ := dialSessionByID(t, server, session.SessionID)
	second, _ := dialSessionByID(t, server, session.SessionID)

	status, _, submitted := submitJob(t, server, session.SessionID, uuid.NewString(), "", "console.log(7)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}

	firstPending := readSocketMessage(t, first)
	secondPending := readSocketMessage(t, second)
	if firstPending.Type != MessageRunStatus || secondPending.Type != MessageRunStatus {
		t.Fatalf("expected run status on both tabs, got %q and %q", firstPending.Type, secondPending.Type)
	}
	if firstPending.JobID != secondPending.JobID || firstPending.JobStatus != "Pending" {
		t.Fatalf("unexpected pending updates: %#v and %#v", firstPending, secondPending)
	}

	jobID, err := uuid.Parse(submitted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	completeJob(t, store, manager, jobID, db.Completion{
		Status:              db.JobSucceeded,
		Result:              encodedResult("7"),
		ResultEncoding:      "base64",
		ResultFormatVersion: 2,
		Output:              "7",
	})

	firstTerminal := readSocketMessage(t, first)
	secondTerminal := readSocketMessage(t, second)
	if firstTerminal.JobStatus != "Success" || secondTerminal.JobStatus != "Success" {
		t.Fatalf("unexpected terminal updates: %#v and %#v", firstTerminal, secondTerminal)
	}
	if firstTerminal.ResultURL == "" || secondTerminal.ResultURL == "" {
		t.Fatal("expected result URLs on both tabs")
	}
}

func TestProcessingStatusIsPublished(t *testing.T) {
	store, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)

	status, _, submitted := submitJob(t, server, sessionID, uuid.NewString(), "", "console.log(8)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	pending := readSocketMessage(t, conn)
	if pending.JobStatus != "Pending" {
		t.Fatalf("expected pending, got %#v", pending)
	}

	jobID, err := uuid.Parse(submitted.JobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}
	processing := markProcessing(t, store, manager, jobID)
	update := readSocketMessage(t, conn)
	if update.JobStatus != "Processing" {
		t.Fatalf("expected processing, got %#v", update)
	}
	if update.Version != processing.Revision {
		t.Fatalf("expected version %d, got %d", processing.Revision, update.Version)
	}
}

func TestSessionReadyCarriesActiveRunMetadata(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	conn, sessionID := dialSession(t, server, manager)
	clientRequestID := uuid.NewString()

	status, _, submitted := submitJob(t, server, sessionID, clientRequestID, "", "console.log(9)")
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	_ = readSocketMessage(t, conn)

	_, ready := dialSessionByID(t, server, sessionID)
	if ready.ActiveRun == nil {
		t.Fatal("expected active run metadata")
	}
	if ready.ActiveRun.JobID != submitted.JobID || ready.ActiveRun.JobStatus != "Pending" {
		t.Fatalf("unexpected active run: %#v", ready.ActiveRun)
	}
	if ready.ActiveRun.ClientRequestID != clientRequestID {
		t.Fatalf("expected client request %q, got %q", clientRequestID, ready.ActiveRun.ClientRequestID)
	}
	if ready.ActiveRun.Version != 1 {
		t.Fatalf("expected version 1, got %d", ready.ActiveRun.Version)
	}
}

func TestResultRouteRejectsMalformedJobID(t *testing.T) {
	_, _, manager := newTestManager(t)
	server := newTestServer(t, manager)
	session, _ := manager.EnsureSession(uuid.NewString())

	response := resultRequest(t, server, server.URL+"/session/"+session.SessionID+"/jobs/not-a-job/result", "identity", "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.StatusCode)
	}
}
