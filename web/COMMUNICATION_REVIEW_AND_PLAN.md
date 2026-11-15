# Communication investigation: review and implementation plan

- **Status:** Phase 1 implemented on `comms-investigation`. Automated backend, frontend, migration, and runtime checks pass; manual browser and deployment measurements remain acceptance work.
- **Date:** 2026-11-14.
- **Reviewed document:** [COMMUNICATION_INVESTIGATION.md](COMMUNICATION_INVESTIGATION.md).
- **Repository baseline:** `comms-investigation` at `c1dc489` (investigation commit), on top of `b033e77`.
- **Implementation scope for the next agent:** Phase 1 only, sections 5 through 11. Phase 2 is a design appendix and must not be started in the same change set without an explicit follow-up decision.

## 0. Executive decision

The investigation's diagnosis is correct: the browser currently downloads a base64-encoded result document whose execution trace dominates the payload, and metadata reads still pull the full result column. The strongest measured improvement is not a new notification transport; it is removing base64 from the browser path, enabling HTTP compression, separating results from status notifications, and stopping metadata queries from reading `result`.

Phase 1 therefore implements:

1. **HTTP job submission and retrieval.** `POST /session/{s}/jobs`, `GET /session/{s}/jobs/{j}`, and `GET /session/{s}/jobs/{j}/result`.
2. **Small WebSocket notifications.** Keep the existing `/ws/{s}` connection and replace the large `run.result` message with a metadata-only `run.status` event.
3. **Compressed raw JSON results.** The result route decodes the stored base64 once and serves the underlying JSON with HTTP gzip when negotiated. The browser never calls `atob()`.
4. **Metadata-only database reads.** History, random-script, and status queries must not select the `result` column.
5. **Processing notifications.** The worker reports `Processing` after claiming a job, not only the terminal state.
6. **Idempotent submission.** A client-generated `client_request_id` plus a database uniqueness constraint makes retries safe. MCP relay submissions use the server-issued execute request ID as that key.
7. **Legacy result support.** Existing base64 successes and text failures remain readable. Phase 1 deliberately keeps new success rows stored as base64 so a rollback to the old backend/frontend remains possible; the browser-facing format changes, the database representation does not.

Phase 1 does **not** implement SSE, Brotli, binary serialization, selective trace loading, direct backend queueing for MCP, per-agent presence leases, authentication, multi-replica fan-out, or live frame streaming.

The target outcome is the investigation's own measurement: the Fibonacci result falls from roughly **65.5 MiB of base64** to roughly **662 KiB of gzipped JSON** on the wire, while WebSocket messages stay small and metadata queries stop transferring the result body.

## 1. Review method

I read the investigation end to end and spot-checked its claims against the baseline source:

- `web/backend/mcpserver/{manager,session,mcp,scripts}.go`
- `web/backend/{db,worker,microvm}` and the migrations
- `web/backend/main.go` and `frontend.go`
- `web/frontend/src/lib/{agentSocket,agentSession,scriptsApi,clientIdentity}.ts`
- `web/frontend/src/routes/{+page.svelte,ExecutionAnimation.svelte}`
- `web/frontend/src/types.ts`
- `runtime/main.go`, `runtime/vm/{debug,trace}.go`, and `runtime/trace/trace.go`

I also ran the baseline backend tests:

```text
cd web/backend && go test ./...
ok  go_js_demo
ok  go_js_demo/mcpserver
```

The full runtime suite (`./test.sh`) is unaffected by this plan and should be run at the end as a regression check.

The investigation's payload measurements are accepted as local payload evidence, not as production latency budgets. Phase 1 must reproduce the size and transport claims on the actual deployment; it does not infer end-to-end latency from the local codec tests.

## 2. Confirmed findings

| Investigation claim | Verification | Phase 1 response |
| --- | --- | --- |
| The browser receives one complete result over WebSocket. | `Session.send` marshals once and enqueues the same bytes to every tab; `handleRunSubmit` eventually calls `completeJob`, which sends `run.result` with the full `result` string. | Replace with metadata-only `run.status`; results move to HTTP. |
| The WebSocket upgrader does not enable compression. | `NewManager` sets only read/write buffer sizes and `CheckOrigin`. | No change needed; the payload no longer belongs on this connection. |
| Structured output is base64-encoded JSON. | `runtime/main.go` marshals `StructuredOut`, base64-encodes it, and prints it. | Decode in the backend/result route; never send base64 to the browser. |
| The trace dominates the result. | `ExecutionInfo.Frames` contains full VM, event-loop, and queue snapshots. The investigation measured about 99.9% of the JSON in the execution section. | Compression solves the Phase 1 network cost. Selective loading is Phase 2. |
| History and random-script queries select `result`. | `jobColumns` includes `result`; `GetJobsBySessionID` and `GetRandomJob` use it. | Add a metadata-only column list and use it for every non-result query. |
| `Processing` is persisted but never pushed. | `GetOldestPendingJob` updates the row; `worker.Run` only sends the terminal `JobResult`. | Worker publishes a `Processing` update after claiming. |
| MCP activity is request-based, not durable presence. | `HandleMCP` counts JSON-RPC-like POST bodies; `beginMCPRequest`/`endMCPRequest` drive the 60-second idle timer. | Keep `waiting`/`active`/`idle` semantics; do not claim durable agent presence. |
| The MCP tool decodes the entire result to extract output. | `decodeOutput` decodes the whole base64 document and unmarshals it into an output struct. | Extract output once in the worker; pass it through the waiter. |
| Reconnect replay is incomplete for fresh pages. | `attach` replays `lastResult` only when idle; `handleRunResult` filters against local active/submitting state. | Snapshot carries metadata only; the frontend fetches the result when it has a matching stored job. |
| Results are scoped by session only implicitly. | `GetJobByJobId` exists but the browser has no single-job route. | Every job/result read must match both `session_id` and `job_id`; return `404` otherwise. |
| SSE is a transport choice, not a payload fix. | The repository uses plain `http.ListenAndServe`; no TLS/HTTP/2 ingress is configured; `Session` connectivity is tied to WebSocket connections. | Retain WebSocket notifications in Phase 1. |

## 3. Corrections and clarifications to the investigation

1. **`format_version: 2` is already the current runtime format.** It is not a new result version introduced by this work. Transport and notification versions are separate from `format_version` and from `execution.schema_version`. Do not bump `format_version` merely because the transport changes.
2. **Stored success and failure results are a mixed representation.** A success is base64 JSON; a failure is plain error text. The implementation must derive behavior from `job_status` and an explicit stored encoding, not from trying base64 decoding first.
3. **The investigation understates the protocol break.** `run.result` is the only path that delivers a result today, and several backend tests and the frontend depend on it. Replacing it with `run.status` is a deliberate breaking change, not an additive message. Update the tests and frontend in the same change set.
4. **`run.accepted` is redundant once submission is HTTP.** The `202` response is the acceptance acknowledgement for the submitting tab; other tabs learn about the job through `run.status` with `job_status: "Pending"`. Do not keep two acceptance messages with different meanings.
5. **The proposed `execute-requests/{r}` source route is not needed in Phase 1.** The execute event already carries at most 1 MiB of source, which is bounded and not the payload problem. Keep the inline code and use the execute request ID for idempotency. A source URL can be added later if late-joining tabs need the editor contents.
6. **SSE is not justified by the current deployment evidence.** Native `EventSource` over HTTP/1.1 shares the browser's roughly six-connection-per-origin limit, and this repository has no verified HTTP/2 ingress. Keeping the existing WebSocket for small notifications captures the measured payload benefit without adding a second connection lifecycle. The event contract below is transport-neutral so SSE can replace WebSocket later.
7. **The current MCP activity heuristic is weaker than the proposed semantics.** `requestHasJSONRPCMethod` counts JSON-RPC-like bodies before SDK validation. Fixing that is a separate MCP correctness task. Phase 1 keeps the existing indicator and the existing user-facing wording; it does not introduce agent leases or presence guarantees.
8. **The investigation's restart discussion needs an explicit Phase 1 answer.** HTTP status makes terminal jobs retrievable after restart, but in-memory session bindings and the worker signal channel are lost. Phase 1 adds a periodic worker poll and a startup stale-processing reconciliation, but does not promise notification delivery for jobs accepted before a restart. Status polling remains the recovery path.
9. **PostgreSQL storage normalization is a Phase 2 concern.** Phase 1 intentionally keeps base64 in the `result` column so that:
   - old success rows remain readable without a backfill;
   - a rollback to the old frontend/backend still works for jobs created during the new deployment;
   - the migration is small and reversible.
   The browser still receives decoded JSON. Phase 2 can normalize storage when it introduces overview/chunk artifacts.
10. **Compression caching is optional, not a correctness requirement.** The measured gzip cost for the Fibonacci capture is about 233 ms per request. Start with per-request gzip and a 300-second private cache header; add an in-process compressed-result cache only if measurements show CPU contention across tabs.
11. **Retention remains three separate policies.** The five-minute last-job replay, the 30-minute session TTL, and persistent history are not unified in Phase 1. Do not add a new expiry contract as part of this change.
12. **Authentication and multi-replica behavior remain out of scope.** The session ID is the capability in the current design. Phase 1 assumes one backend instance. Cross-instance event fan-out is a separate architecture task.

## 4. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Migration breaks old rows or rollback. | Add columns with safe defaults; keep success storage base64; derive failure behavior from `job_status`; add a down migration. |
| Duplicate submissions create duplicate jobs. | `UNIQUE (session_id, client_request_id)`; `InsertNewJob` is an upsert-or-return; same key with different source returns `409`. |
| Two tabs submit the same MCP request concurrently. | Serialize session submissions with `submitMu`; first takes the pending waiter; second returns the existing job. |
| A result is advertised before it is persisted. | Worker persists the terminal row first, then publishes `run.status`; the event's `result_url` is only set for a committed success. |
| A terminal event is lost. | Every job has a status URL; the frontend polls when the WebSocket is disconnected or when a submission response is lost. `session.ready` carries metadata for reconciliation. |
| The status/history route still reads a 50 MiB result. | Use a metadata-only column list; add tests that fake the store's metadata path and review every SQL query. |
| Gzip CPU becomes a bottleneck. | Measure; use a private cache header and ETag; add a bounded compressed-result cache only if needed. |
| A backend restart loses in-memory bindings. | Worker poll picks up pending rows; startup marks stale `Processing` rows failed; frontend status polling recovers terminal rows. Document the notification gap. |
| The browser still parses a 50 MiB JSON document. | Measure in Phase 1. Phase 2 introduces overview and trace chunks. Do not pretend compression solves parse/memory cost. |
| Rollback after new jobs. | New success rows remain base64, so the old backend/frontend path still works. The new columns are additive. |

## 5. Decisions requested by the investigation

### D1. Notification transport: retain WebSocket; defer SSE

**Decision:** Keep one WebSocket per tab for small notifications in Phase 1. Do not add SSE.

**Rationale:** The measured benefit comes from removing the result body, not from changing transports. The deployed connection is plain HTTP/1.1, and native SSE would add HTTP/1.1 connection pressure while the session registry and MCP workflow are already WebSocket-bound. A later SSE migration can reuse the `run.status` event contract.

**Consequence:** Notifications still require a live WebSocket for immediate updates. The status endpoint and polling fallback make correctness independent of the notification.

### D2. Agent semantics: recent/in-flight activity, not durable presence

**Decision:** Keep `waiting`, `active`, and `idle` as "recent or in-flight MCP activity". Do not add leases, heartbeats, or per-agent identity in Phase 1.

**Consequence:** The UI continues to say "Agent active" and "Last active", never "Agent connected".

### D3. Execution coordination: preserve the browser relay

**Decision:** Keep the MCP `execute.request` -> tab -> HTTP submission relay. Do not queue MCP executions directly in the backend.

**Rationale:** Direct queueing changes the documented product behavior (the tab must be open) and the cancellation semantics. The transport change should not change who owns execution. Use the server-issued execute request ID as the idempotency key so multiple tabs cannot execute the same request twice.

**Consequence:** MCP still requires a connected frontend. `execute.rejected` remains the rejection path. A source URL for late-joining tabs is deferred.

### D4. Artifact contract: full compressed result now, selective trace later

**Decision:** Phase 1 serves the complete result as one compressed JSON document over HTTP. Phase 2 introduces overview plus independently loadable trace chunks. Do not silently return an incomplete `InterpretDetails` object.

**Consequence:** The animation still materializes the full frame array in Phase 1. The measured network cost is fixed; the parse/memory cost is explicitly deferred and must be measured before Phase 2.

### D5. Compression/format: JSON + gzip baseline only

**Decision:** Use raw JSON with gzip content encoding. Do not add Brotli, MessagePack, CBOR, Protocol Buffers, FlatBuffers, or live streaming in Phase 1.

**Rationale:** The investigation's own measurements show that binary formats largely lose their advantage after compression. Gzip is in the Go standard library and is supported by every target browser. Brotli remains a candidate after real deployment measurements.

### D6. Deployment and recovery scope: single instance, status-authoritative

**Decision:** Phase 1 targets the current single backend instance. The database is authoritative for job status and results. Notifications are hints. A lost notification must never make a result unrecoverable while the row exists.

**Consequence:** Multi-replica fan-out, shared event state, and cross-instance session affinity are out of scope.

## 6. Phase 1 target architecture

```text
Browser
  -> POST /session/{s}/jobs {client_request_id, code}
  -> Session.Submit validates, inserts Pending row, binds job, sends WS run.status(Pending)
  -> HTTP 202 {job_id, status_url, version, created}
  -> Worker claims row, sets Processing, sends WS run.status(Processing)
  -> Firecracker VM runs the interpreter (unchanged base64 output)
  -> Worker decodes/validates JSON, extracts output, persists terminal row (base64 success or text failure)
  -> Worker sends terminal update; manager sends WS run.status(Success/Failed, result_url, version)
  -> Browser GETs result_url
  -> Backend decodes stored base64, serves raw JSON, gzip if negotiated
```

Components and responsibilities:

- **`Session`** remains the per-session coordinator: one active run, one pending MCP request, WebSocket fan-out, MCP waiter.
- **`Manager`** owns the session registry and job-to-session bindings. It no longer carries result bodies.
- **`Worker`** owns VM execution, result validation, persistence, and status updates.
- **`db`** exposes metadata queries separately from result queries.
- **HTTP job handlers** enforce session scoping, idempotency, compression, and error contracts.
- **Frontend** treats HTTP as the source of truth and WebSocket as a notification hint.

## 7. Data model and migration

Add `web/backend/db/migrations/000003_add_job_metadata.up.sql`:

```sql
ALTER TABLE user_script_jobs
    ADD COLUMN IF NOT EXISTS client_request_id text,
    ADD COLUMN IF NOT EXISTS result_encoding text NOT NULL DEFAULT 'base64',
    ADD COLUMN IF NOT EXISTS result_format_version integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS completed_at timestamp WITH TIME ZONE;

CREATE UNIQUE INDEX IF NOT EXISTS user_script_jobs_session_client_request_idx
    ON user_script_jobs (session_id, client_request_id);

CREATE INDEX IF NOT EXISTS user_script_jobs_status_created_at_idx
    ON user_script_jobs (job_status, created_at);
```

Down migration:

```sql
DROP INDEX IF EXISTS user_script_jobs_status_created_at_idx;
DROP INDEX IF EXISTS user_script_jobs_session_client_request_idx;

ALTER TABLE user_script_jobs
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS revision,
    DROP COLUMN IF EXISTS output,
    DROP COLUMN IF EXISTS result_format_version,
    DROP COLUMN IF EXISTS result_encoding,
    DROP COLUMN IF EXISTS client_request_id;
```

Field semantics:

| Column | Meaning |
| --- | --- |
| `client_request_id` | Client-generated idempotency key, unique per session. `NULL` for legacy rows. |
| `result_encoding` | `base64` for success documents in Phase 1, `text` for failure messages, `json` reserved for future raw storage. Existing rows default to `base64`; failure behavior is determined by `job_status`, not this column alone. |
| `result_format_version` | Parsed `format_version` from a successful structured result; `0` for legacy/unknown. |
| `output` | Program output extracted once at completion. Used by MCP so it never decodes the full trace. |
| `revision` | Monotonic job-state version starting at `1`; incremented on every status change. |
| `completed_at` | Set when the job reaches `Success` or `Failed`. |

PostgreSQL allows multiple `NULL`s in a unique index, so the unique `(session_id, client_request_id)` index is safe for legacy rows.

### 7.1 Query separation

Introduce two column lists in `web/backend/db/db.go`:

```go
const jobMetadataColumns = `
    job_id,
    COALESCE(session_id::text, '') AS session_id,
    src,
    job_status,
    COALESCE(client_request_id, '') AS client_request_id,
    COALESCE(result_encoding, 'base64') AS result_encoding,
    COALESCE(result_format_version, 0) AS result_format_version,
    COALESCE(output, '') AS output,
    revision,
    created_at,
    updated_at,
    completed_at`

const jobResultColumns = jobMetadataColumns + `, result`
```

Every query except the single-job result lookup must use `jobMetadataColumns`. Specifically:

- `GetJobsBySessionID` (history) must not select `result`.
- `GetRandomJob` must not select `result`.
- `GetJobBySessionID` (status) must not select `result`.
- `GetJobResultBySessionAndID` is the only query allowed to select `result`.

`src` remains in history because the UI loads a selected script from it. It is bounded by the 1 MiB submission limit and is not the measured result payload. Returning a preview and fetching source on selection is a future optimization, not Phase 1.

### 7.2 `db.UserScriptJob`

Extend the struct without exposing the result accidentally:

```go
type UserScriptJob struct {
    JobID               uuid.UUID  `json:"job_id"`
    SessionID           string     `json:"session_id"`
    Src                 string     `json:"src"`
    JobStatus           JobStatus  `json:"job_status"`
    Result              string     `json:"-"`
    ResultEncoding      string     `json:"-"`
    ResultFormatVersion int        `json:"-"`
    Output              string     `json:"-"`
    ClientRequestID     string     `json:"client_request_id,omitempty"`
    Revision            int64      `json:"revision"`
    CreatedAt           time.Time  `json:"created_at"`
    UpdatedAt           time.Time  `json:"updated_at"`
    CompletedAt         *time.Time `json:"completed_at,omitempty"`
}
```

Add sentinel errors:

```go
var (
    ErrJobNotFound           = errors.New("job not found")
    ErrClientRequestConflict = errors.New("client_request_id already used with different source")
)
```

Add completion struct:

```go
type Completion struct {
    Status              JobStatus
    Result              string
    ResultEncoding      string
    ResultFormatVersion int
    Output              string
}
```

## 8. Backend implementation

### 8.1 New `jobresult` package

Create `web/backend/jobresult/jobresult.go`. It is the only place that understands the runtime's base64 structured output.

```go
package jobresult

type Decoded struct {
    JSON          []byte
    FormatVersion int
    Output        string
}

func Decode(encoded string) (*Decoded, error)
```

`Decode` must:

1. Trim surrounding whitespace.
2. Base64-decode with `base64.StdEncoding`.
3. Unmarshal only the small envelope fields:
   ```go
   var envelope struct {
       FormatVersion int    `json:"format_version"`
       Output        string `json:"output"`
   }
   ```
   A successful `json.Unmarshal` validates the whole document, including the trace, without materializing it. Do not call `json.Valid` first and do not unmarshal into `map[string]any` or the full `StructuredOut`; those would add a second scan or materialize the trace unnecessarily.
4. Require `FormatVersion == 2` in Phase 1. Return a descriptive error otherwise.
5. Return the decoded JSON bytes, format version, and output.

Also provide:

```go
func LegacyJSON(stored string) ([]byte, error)
```

for the result handler. It base64-decodes legacy success rows, trims whitespace, and returns the bytes. The handler performs the single `json.Valid` check. A decode failure means the stored success row is corrupt; the handler returns `500`.

### 8.2 Database methods

Update the `JobStore` interface in `web/backend/mcpserver/session.go` to:

```go
type JobStore interface {
    InsertNewJob(sessionID, clientRequestID, src string, ctx context.Context) (*db.UserScriptJob, bool, error)
    GetJobsBySessionID(sessionID string, ctx context.Context) ([]db.UserScriptJob, error)
    GetRandomJob(ctx context.Context) (*db.UserScriptJob, error)
    GetJobBySessionAndID(sessionID string, jobID uuid.UUID, ctx context.Context) (*db.UserScriptJob, error)
    GetJobResultBySessionAndID(sessionID string, jobID uuid.UUID, ctx context.Context) (*db.UserScriptJob, error)
    GetJobBySessionAndClientRequestID(sessionID, clientRequestID string, ctx context.Context) (*db.UserScriptJob, error)
}
```

The worker continues to use `*db.Db` directly for `MarkOldestPendingJobProcessing` and `UpdateJob`; those are execution concerns, not session/HTTP concerns.

Implementations:

- **`InsertNewJob`**
  - Parse `sessionID` as UUID.
  - Treat an empty `clientRequestID` as `NULL` (the HTTP API requires non-empty; this is only for internal/legacy callers).
  - Use:
    ```sql
    INSERT INTO user_script_jobs (session_id, src, job_status, client_request_id)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (session_id, client_request_id) DO NOTHING
    RETURNING <metadata columns>
    ```
  - If `pgx.ErrNoRows`, select the existing row by `(session_id, client_request_id)`.
  - If the existing row's `src` differs from the submitted source, return `ErrClientRequestConflict`.
  - Return the row and `created=true` for the insert, `created=false` for the existing row.
- **`MarkOldestPendingJobProcessing`**
  - Rename/replace `GetOldestPendingJob`.
  - Same `UPDATE ... FOR UPDATE SKIP LOCKED` shape, but set `revision = revision + 1`.
  - Return metadata columns only.
- **`UpdateJob`**
  - Accept `db.Completion`.
  - Set `job_status`, `result`, `result_encoding`, `result_format_version`, `output`, `revision = revision + 1`, `updated_at = NOW()`, `completed_at = NOW()`.
  - Return metadata columns only.
  - Map zero rows to `ErrJobNotFound`.
- **`GetJobBySessionAndID`**
  - `WHERE job_id = $1 AND session_id = $2`.
  - Select metadata columns only.
  - `pgx.ErrNoRows` -> `ErrJobNotFound`.
- **`GetJobResultBySessionAndID`**
  - Same predicate, select result columns.
  - Used only by the result handler.
- **`GetJobBySessionAndClientRequestID`**
  - `WHERE session_id = $1 AND client_request_id = $2`.
  - Select metadata columns plus `src` (already in metadata).
  - Used for late/retried MCP execute submissions.
- **`GetJobsBySessionID`** and **`GetRandomJob`**
  - Switch to metadata columns; remove `result` from scans.
- **`FailStaleProcessing`**
  - Add:
    ```go
    func (db *Db) FailStaleProcessing(before time.Time, reason string, ctx context.Context) (int64, error)
    ```
  - Set `Failed`, `result_encoding='text'`, `result=reason`, `revision=revision+1`, `completed_at=NOW()` for rows in `Processing` with `updated_at < before`.
  - Call it once at startup with a generous threshold (10 minutes). This is not a substitute for exactly-once recovery; it prevents permanently stuck rows after a crash.

### 8.3 Worker changes

Update `web/backend/worker/worker.go`:

```go
type JobResult struct {
    SessionID       string
    ClientRequestID string
    JobID           uuid.UUID
    Status          db.JobStatus
    Revision        int64
    Output          string
    ErrorMessage    string
}
```

`SessionID` and `ClientRequestID` come from the metadata returned by `MarkOldestPendingJobProcessing`. They are required so an update can still reach the right session and the right tab if the worker claims the job before `Session.Submit` has finished binding it.

Add a poll fallback so pending rows survive a lost signal or backend restart:

```go
const pollInterval = 2 * time.Second
```

`Worker.Run` must:

1. Try `processOne` immediately.
2. `select` on `w.sig`, `ticker.C`, and `ctx.Done()`.
3. Process at most one job per wake-up. `FOR UPDATE SKIP LOCKED` makes concurrent workers safe.

`processOne`:

1. `job, err := w.db.MarkOldestPendingJobProcessing(ctx)`.
2. On `pgx.ErrNoRows`, return.
3. Publish `JobResult{SessionID: job.SessionID, ClientRequestID: job.ClientRequestID, JobID: job.JobID, Status: db.JobProcessing, Revision: job.Revision}`. This intermediate notification is best-effort: use a non-blocking send and log a full channel. The terminal update must not be dropped; if the results channel is full, wait for capacity or the context.
4. Run `w.mvm.RunCode(job.Src, ctx)`.
5. Build a `db.Completion`:
   - Success:
     ```go
     decoded, decodeErr := jobresult.Decode(res)
     if decodeErr != nil {
         completion = db.Completion{
             Status:         db.JobFailed,
             Result:         "invalid structured result: " + decodeErr.Error(),
             ResultEncoding: "text",
         }
     } else {
         completion = db.Completion{
             Status:              db.JobSucceeded,
             Result:              res,          // keep Phase 1 storage as base64
             ResultEncoding:      "base64",
             ResultFormatVersion: decoded.FormatVersion,
             Output:              decoded.Output,
         }
     }
     ```
   - Runtime error:
     ```go
     completion = db.Completion{
         Status:         db.JobFailed,
         Result:         runErr.Error(),
         ResultEncoding: "text",
     }
     ```
6. `updated, err := w.db.UpdateJob(job.JobID, completion, ctx)`.
7. If the update fails:
   - Log the original error.
   - Attempt one fallback `UpdateJob` with `Failed`, `"failed to persist job result: ..."`, encoding `text`.
   - If the fallback succeeds, use its revision and failure status.
   - If the fallback fails, publish a failure with the last known revision and do not claim success.
8. Publish the terminal `JobResult` only after the database commit returns. Include `SessionID`, `ClientRequestID`, `Output` on success, and `ErrorMessage` on failure.

Do not publish a success before `UpdateJob` returns.

### 8.4 Session and manager changes

#### Session fields

Extend `activeRun` and replace `lastResult` with a metadata-only last-job record:

```go
type activeRun struct {
    requestID       string
    clientRequestID string
    jobID           uuid.UUID
    status          db.JobStatus
    revision        int64
    waiter          chan runOutcome
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
```

Remove `recentRequests`; the database uniqueness constraint is authoritative.

Add `submitMu sync.Mutex` to `Session`. Lock order is always `submitMu` then `mu`. `Run` (MCP) takes `submitMu` only while registering/clearing the pending request, not while waiting.

#### `Session.Submit`

Add:

```go
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

func (s *Session) Submit(ctx context.Context, req SubmitRequest) (SubmitResult, error)
```

Algorithm:

1. Validate `len(req.Code) <= maxCodeSize` and a non-empty `ClientRequestID` of at most 128 characters.
2. Acquire `submitMu` for the whole state-transition and insert sequence.
3. Under `s.mu`:
   - If closed, return `ErrSessionClosed`.
   - If `s.active != nil`:
     - If `req.ExecuteRequestID` matches `active.requestID`, or `req.ClientRequestID` matches `active.clientRequestID`, return the current job metadata as an idempotent success.
     - Otherwise return `ErrSessionBusy`.
   - If `req.ExecuteRequestID != ""`:
     - If `s.pendingMCP == nil` or its request ID differs:
       - Unlock and query `GetJobBySessionAndClientRequestID`.
       - If found and source matches, return it.
       - If found and source differs, return `ErrClientRequestConflict`.
       - If the lookup returns `ErrJobNotFound`, return `ErrExecuteRequestExpired`.
       - Propagate any other storage error.
     - Take `pendingMCP.waiter`, set `requestID = pendingMCP.requestID`, clear `pendingMCP`.
   - Else:
     - If `s.pendingMCP != nil`, return `ErrAgentPending`.
     - Generate `requestID = uuid.NewString()`.
4. Unlock `s.mu`; call `store.InsertNewJob(s.id, req.ClientRequestID, req.Code, ctx)`.
5. If the insert fails, resolve any captured MCP waiter with the error and return.
6. If the returned job is terminal, resolve a captured MCP waiter with the terminal outcome and return the job without setting `active`.
7. Under `s.mu`:
   - Recheck `s.closed`.
   - If `s.lastJob` now describes the same job ID (the worker completed before the binding was installed), use that completed record as the result, resolve the captured MCP waiter, and return without setting `active`.
   - If `s.active` already exists for the same job ID (the worker published `Processing` before the binding was installed), attach the captured waiter and `requestID` when they are not already set, and adopt the higher status/revision.
   - Otherwise set `s.active = &activeRun{...}` with the returned job's status/revision.
8. `mgr.bindJob(s, job.JobID)`.
9. Send `run.status` with the current status/revision to all tabs. This is `Pending` normally, `Processing` if step 7 adopted an early update, or the terminal status if step 7 found a completed record. The `SubmitResult` returned to HTTP must carry the same adopted status/revision.
10. Best-effort signal the worker:
    ```go
    select {
    case s.work <- worker.HeartbeatSignal{}:
    default:
        // The periodic worker poll is the fallback. Do not fail the job.
    }
    ```
11. Return `SubmitResult{Job: *job, RequestID: requestID, Created: created}`.

`Run` (MCP) must use `submitMu` while checking `active`/`pendingMCP`, creating the waiter, and sending `execute.request`. It must not hold `submitMu` while waiting for the outcome.

#### `Session` status updates

Add:

```go
func (s *Session) applyJobUpdate(update worker.JobResult)
```

Behavior:

- Under `s.mu`, find `active` by job ID.
- If the update is `Processing`:
  - If it matches `active`, update `active.status`/`revision`.
  - Build a `run.status` event using `request_id`/`client_request_id` from `active` when available, otherwise from the worker update.
- If terminal:
  - Capture `active` if it matches; clear it.
  - Always store a metadata-only `completedRun` for the five-minute replay, even when no `active` matches.
  - Build a `run.status` event with `job_id`, `job_status`, `version`, `result_url` on success, and `error` on failure. Use `client_request_id` from the worker update so a tab that is still waiting for its POST response can recognize the job.
- If no `active` matches and the update is `Processing`, create an `activeRun` from the update with a nil waiter, so snapshots and a still-in-flight `Submit` see the real state instead of a stale `Pending`.
- If no `active` matches, still send the metadata-only event. This keeps other tabs and post-restart sessions informed.
- Unlock before calling `s.send`.
- Resolve the waiter with `JobID`, `Status`, `Output`, `ErrorMessage`, `Revision` only when an active run with a waiter was captured.

`completeJob`, `failActive`, and `failActiveWithJob` must be rewritten in terms of this update path. Do not send the result body.

The worker can claim a row between the database insert and `Session.Submit` setting `s.active`. The `completedRun` record closes that race: after the insert, `Submit` rechecks `lastJob` for the same job ID under `s.mu` before setting `active`; if the job already finished, it returns the terminal metadata and never publishes a stale `Pending`. A `Processing` update that arrives before `active` exists is still sent to the browser with its `client_request_id`, so the tab can adopt it.

#### `Session.attach` snapshot

`session.ready` must include:

- MCP status and last-seen.
- `active_run` metadata: `request_id`, `client_request_id`, `job_id`, `job_status`, `version`.
- `last_job` metadata only when there is no active run and the last completed job is within `lastResultRetention`: `job_id`, `client_request_id`, `job_status`, `version`, `result_url` (success), `error` (failure). This matches the current replay rule.
- The pending `execute.request` as today.

It must not include a result body.

#### Manager

- `DeliverJobResult` becomes `DeliverJobUpdate(update worker.JobResult)`.
- Look up the binding by job ID.
- If a binding exists, call `session.applyJobUpdate(update)` and delete the binding only on `Success`/`Failed`.
- If no binding exists and `update.SessionID` is non-empty, call `m.getOrCreateSession(update.SessionID)`, bind the job for a non-terminal update, and call `applyJobUpdate`. This is required because the worker poll can claim a row before `Session.Submit` reaches `bindJob`.
- If no binding exists and `update.SessionID` is empty, log and drop; status polling remains authoritative.
- `main.go` loop passes the new struct.

#### Protocol

`web/backend/mcpserver/protocol.go`:

- Keep: `session.ready`, `execute.request`, `execute.rejected`, `mcp.status`.
- Add: `run.status`.
- Remove: `run.submit`, `run.accepted`, `run.result`, `run.error`.
- Remove the `Result` field from `socketMessage`.
- Add `ClientRequestID`, `Version`, `ResultURL`, `ActiveRun`, `LastJob`.

Suggested message shape:

```go
type socketMessage struct {
    Type            string          `json:"type"`
    SessionID       string          `json:"session_id,omitempty"`
    RequestID       string          `json:"request_id,omitempty"`
    ClientRequestID string          `json:"client_request_id,omitempty"`
    JobID           string          `json:"job_id,omitempty"`
    Code            string          `json:"code,omitempty"`
    Error           string          `json:"error,omitempty"`
    JobStatus       string          `json:"job_status,omitempty"`
    Version         int64           `json:"version,omitempty"`
    ResultURL       string          `json:"result_url,omitempty"`
    ActiveRun       *activeRunInfo  `json:"active_run,omitempty"`
    LastJob         *jobStatusInfo  `json:"last_job,omitempty"`
    MCPStatus       string          `json:"mcp_status,omitempty"`
    MCPLastSeen     string          `json:"mcp_last_seen,omitempty"`
}
```

### 8.5 HTTP handlers

Add `web/backend/mcpserver/jobs.go` and register:

```go
mux.HandleFunc("POST /session/{session_id}/jobs", mgr.HandleCreateJob)
mux.HandleFunc("GET /session/{session_id}/jobs/{job_id}", mgr.HandleJobStatus)
mux.HandleFunc("GET /session/{session_id}/jobs/{job_id}/result", mgr.HandleJobResult)
```

Also add these routes to `newTestServer` in `session_test.go`.

All handlers must:

- Canonicalize the session ID.
- Parse the job ID as UUID; invalid -> `404`.
- Return `503 storage_unavailable` when `m.store == nil`.
- Call `session.touch()` so HTTP polling keeps the session alive.
- Set `Cache-Control: no-store` on JSON status/error responses.
- Enforce session scoping through the store methods.
- Log job ID, status, revision, and result byte size only. Never log the result body or submitted source.

#### `HandleCreateJob`

Request:

```json
{
  "client_request_id": "8d0f...",
  "execute_request_id": "optional-server-issued-id",
  "code": "console.log(2)"
}
```

Rules:

- Limit the body with `http.MaxBytesReader` to `maxCodeSize + 16*1024`.
- `client_request_id` is required, trimmed, 1-128 characters.
- `code` length is checked against `maxCodeSize`.
- `execute_request_id` is optional. When present it must equal `client_request_id` and must match a pending MCP request or an already accepted job. Reject a mismatch with `400 invalid_request`.
- Call `session.Submit`.
- Return `202 Accepted` with `Location: /session/{s}/jobs/{j}`.

Response:

```json
{
  "job_id": "…",
  "request_id": "…",
  "client_request_id": "…",
  "job_status": "Pending",
  "version": 1,
  "created": true,
  "status_url": "/session/{s}/jobs/{j}",
  "result_url": null
}
```

For an idempotent retry, return the existing job's current status and `created: false`. If it is already successful, include `result_url`.

Error mapping:

| Error | Status | Code |
| --- | --- | --- |
| Invalid JSON/body/IDs | `400` | `invalid_request` |
| Code too large | `413` | `code_too_large` |
| `ErrSessionBusy` | `409` | `session_busy` |
| `ErrAgentPending` | `409` | `agent_request_pending` |
| `ErrExecuteRequestExpired` | `410` | `execute_request_expired` |
| `ErrClientRequestConflict` | `409` | `idempotency_conflict` |
| `ErrSessionClosed` | `503` | `session_closed` |
| Store unavailable | `503` | `storage_unavailable` |

#### `HandleJobStatus`

- Load via `GetJobBySessionAndID`.
- Return `200` for every known job, including `Failed`.
- `404` for unknown or differently scoped jobs.

Response:

```json
{
  "job_id": "…",
  "client_request_id": "…",
  "job_status": "Success",
  "version": 3,
  "created_at": "2026-11-14T12:00:00Z",
  "updated_at": "2026-11-14T12:00:02Z",
  "completed_at": "2026-11-14T12:00:02Z",
  "result_url": "/session/{s}/jobs/{j}/result",
  "error": null
}
```

For a failure, omit `result_url` and include:

```json
{
  "error": {
    "code": "execution_failed",
    "message": "the exact stored failure text"
  }
}
```

Never include the stored result body.

#### `HandleJobResult`

- Load via `GetJobResultBySessionAndID`.
- Pending/Processing -> `202` with `Retry-After: 1`, status metadata, and `Location` to the status URL.
- Failed -> `409` with `execution_failed` and the stored message. A failed job is a successful status lookup, but the result document does not exist.
- Success -> decode and serve.

Success response rules:

- `Content-Type: application/json; charset=utf-8`.
- `Cache-Control: private, max-age=300`.
- `Vary: Accept-Encoding`.
- `ETag` must identify the selected representation, for example `"<job_id>-<revision>-gzip"` or `"<job_id>-<revision>-identity"`. Do not reuse one strong validator across gzip and identity bodies.
- If `If-None-Match` matches the selected representation's ETag, return `304 Not Modified` with the same `Vary`, `ETag`, and `Cache-Control`.
- If the client accepts gzip, set `Content-Encoding: gzip` and write with `compress/gzip` at `gzip.DefaultCompression`.
- Otherwise write identity bytes.
- Decode success storage:
  - `result_encoding == "json"`: use the stored text directly (future).
  - `result_encoding == "base64"` or empty: `jobresult.LegacyJSON(job.Result)`.
  - Any other encoding: `500 invalid_stored_result`.
- Validate the decoded bytes with `json.Valid`; invalid -> `500`.
- Never wrap the JSON in a JSON string.

Compression negotiation helper:

```go
func acceptsGzip(header string) bool
```

It must parse comma-separated codings and `q` values. Explicit `gzip;q=0` disables gzip even if `*` is present. Absent header means identity.

Error body:

```json
{
  "error": {
    "code": "execution_failed",
    "message": "…"
  }
}
```

### 8.6 MCP integration

`runOutcome` gains:

```go
Output       string
ErrorMessage string
Revision     int64
```

`mcp.go`:

- On success, return `outcome.Output` and `outcome.JobID`.
- On failure, return an error containing `outcome.ErrorMessage`.
- Delete `decodeOutput`; the worker already extracted output.

The MCP tool continues to return only output and job ID. The full result is never sent through MCP.

### 8.7 Main wiring

- Call `database.FailStaleProcessing(ctx, time.Now().Add(-10*time.Minute), "interrupted by backend restart")` before starting workers. Log the count and any error.
- Start the worker pool with the updated result channel.
- Register the three job routes.
- Keep the existing `/session/{session_id}/jobs` history route and `/scripts/random`.

## 9. API and event contract reference

### 9.1 Job states

```text
Pending -> Processing -> Success
   |            |
   +----------> Failed
```

- `Pending`: durably accepted into the queue.
- `Processing`: claimed by a worker, including VM startup. Not instruction-level progress.
- `Success`: terminal row committed and result fetchable.
- `Failed`: terminal row committed with a stored failure message.

`version` is the database `revision` and is monotonic per job. It is not a trace `seq` and not `execution.schema_version`.

`request_id` is an in-memory correlation ID owned by the session. `client_request_id` is the durable idempotency key and is what clients must use to correlate retries, reloads, and events. `GET /jobs/{j}` does not return `request_id` because it is not persisted; events and `session.ready` include it only when the session still remembers the active run.

### 9.2 `run.status` event

```json
{
  "type": "run.status",
  "job_id": "…",
  "request_id": "…",
  "client_request_id": "…",
  "job_status": "Success",
  "version": 3,
  "result_url": "/session/{s}/jobs/{j}/result"
}
```

Failure:

```json
{
  "type": "run.status",
  "job_id": "…",
  "job_status": "Failed",
  "version": 3,
  "error": "the stored failure message"
}
```

Rules:

- `Pending` is emitted once after the insert commits.
- `Processing` is emitted after the worker claims the row.
- Terminal is emitted only after the terminal row commits.
- Events are idempotent. Clients apply the highest `version` per job and ignore older versions.
- A tab that is not displaying a job may still update its history list.

### 9.3 `session.ready` snapshot

```json
{
  "type": "session.ready",
  "session_id": "…",
  "mcp_status": "active",
  "active_run": {
    "request_id": "…",
    "client_request_id": "…",
    "job_id": "…",
    "job_status": "Processing",
    "version": 2
  },
  "last_job": {
    "job_id": "…",
    "client_request_id": "…",
    "job_status": "Success",
    "version": 3,
    "result_url": "/session/{s}/jobs/{j}/result"
  }
}
```

`last_job` is only sent when there is no `active_run`; the frontend must handle both being absent.

## 10. Frontend implementation

### 10.1 New `jobsApi.ts`

Create `web/frontend/src/lib/jobsApi.ts`:

```ts
export type JobStatus = "Pending" | "Processing" | "Success" | "Failed";

export type JobStatusResponse = {
    job_id: string;
    request_id?: string;
    client_request_id?: string;
    job_status: JobStatus;
    version: number;
    created?: boolean;
    status_url?: string;
    result_url?: string | null;
    error?: { code: string; message: string } | null;
    created_at?: string;
    updated_at?: string;
    completed_at?: string | null;
};

export class JobApiError extends Error {
    status: number;
    code?: string;
}

export async function submitJob(
    sessionId: string,
    request: {
        code: string;
        client_request_id: string;
        execute_request_id?: string;
    },
): Promise<JobStatusResponse>;

export async function fetchJobStatus(
    sessionId: string,
    jobId: string,
): Promise<JobStatusResponse>;

export async function fetchJobResult(
    resultUrl: string,
): Promise<unknown>;
```

`fetchJobResult` must:

- Use `fetch(resultUrl, { headers: { Accept: "application/json" } })`.
- On `200`, return `response.json()`.
- On `202`, throw a typed pending error carrying `Retry-After`.
- On `409`, parse the error envelope and throw `JobApiError` with code `execution_failed`.
- On other statuses, throw `JobApiError` with the response text.

Use `encodeURIComponent` for session/job IDs when constructing URLs.

### 10.2 WebSocket types

Update `agentSocket.ts`:

- Remove `run.accepted`, `run.result`, `run.error`, and `run.submit`.
- Add `run.status`.
- Keep `session.ready`, `mcp.status`, `execute.request`, `execute.rejected`.
- Add `client_request_id`, `version`, `result_url`, `active_run`, and `last_job` fields.
- Keep `ConnectionStatus`.

### 10.3 Page state

Add a `"loading"` page state for the period after a terminal success event but before the result document is parsed. Update the status label:

- `submitting`: "Submitting…"
- `running`: "Running…"
- `loading`: "Loading result…"
- `done`: existing behavior
- `error`: existing behavior

`isBusy` includes `submitting`, `running`, and `loading`.

Track:

```ts
let activeJob = $state<{
    jobId: string;
    requestId: string;
    clientRequestId: string;
    version: number;
    status: JobStatus;
} | null>(null);
let pendingClientRequestId: string | null = null;
let jobPollTimer: ReturnType<typeof setTimeout> | null = null;
let resultAbort: AbortController | null = null;
```

Keep or replace the existing `jobId`/`activeRequestId` variables; the important rule is one source of truth for the active job.

### 10.4 Submission

`sendRun(code, executeRequestId?)` becomes an async function returning `Promise<boolean>`:

1. Reject if `activeJob !== null` or `pageState` is `submitting`, `running`, or `loading`. A client timeout must not discard the job identity.
2. Generate `clientRequestId = executeRequestId ?? crypto.randomUUID()`.
3. Set `pendingClientRequestId`, clear the active job/result/error, set `pageState = "submitting"`, and arm the existing 120-second client timeout.
4. Write `{client_request_id, request_id}` to `sessionStorage` under `go-js-active-job-v1`, then add `job_id` as soon as the HTTP response or a `run.status` event provides it.
5. Call `submitJob(sessionId, {code, client_request_id, execute_request_id})`.
6. On success, call `applyJobStatus(response, {submittedByThisTab: true})`.
7. On `JobApiError`, if a `run.status` event already adopted this `client_request_id`, keep the adopted job and start/continue polling instead of reporting a submission failure. Otherwise show the message, set `pageState = "error"`, clear `pendingClientRequestId` and the sessionStorage entry, and stop polling. If this was an MCP execute submission, send `execute.rejected` with the message so the MCP waiter can resolve; the backend ignores it if another tab already accepted the job.

Do **not** require `serverStatus === "connected"` to submit. HTTP availability is what matters. Start status polling if the WebSocket is not connected.

### 10.5 `run.status` handling

`handleRunStatus(message)`:

1. If `message.client_request_id` matches `pendingClientRequestId`, adopt the job even if the HTTP response has not arrived. This handles completion before the POST reply.
2. Otherwise, accept only if `activeJob?.jobId === message.job_id`.
3. Apply the message only if `message.version >= activeJob.version` (strictly greater for state changes; equal is an idempotent duplicate).
4. On `Pending`/`Processing`: set `pageState = "running"`; refresh history on `Pending`.
5. On `Success`: set `pageState = "loading"`, clear the timeout, refresh history, and call `loadJobResult(jobId, result_url)`.
6. On `Failed`: set `runError` from `message.error`, set `interpretResult` to a failed placeholder, clear the active job, set `pageState = "done"`, and refresh history.

`applyJobStatus(response)` follows the same state transitions for the HTTP response. If a `run.status` event with a higher version already arrived, the HTTP response is ignored.

### 10.6 Result loading

`loadJobResult(jobId, resultUrl)`:

1. Abort any previous result fetch.
2. Use `resultUrl` if present; otherwise construct `/session/{s}/jobs/{j}/result`.
3. Set `pageState = "loading"`.
4. Call `fetchJobResult`.
5. On success:
   - Treat the parsed object as `InterpretDetails`.
   - Set `debugChunks`, `interpretResult`, `lookUp`, `outputExpanded`, and `resultView` exactly as the current code does.
   - Set `pageState = "done"` and clear `pendingClientRequestId`.
6. On a pending `202`:
   - Wait for `Retry-After` (default 250 ms), then retry up to five times. This is a safety net; a committed success should not normally return `202`.
7. On `execution_failed`:
   - Show the error and set a failed `interpretResult`.
8. On network/parse failure:
   - Show a clear error; keep the job ID in sessionStorage so the user can reload and retry.

Remove `atob()` entirely. Browser `response.json()` handles UTF-8, including chess and block characters.

### 10.7 Polling fallback

If `serverStatus !== "connected"` after a submission, if the WebSocket disconnects while a job is active, or if a submission response is lost:

- Poll `fetchJobStatus(sessionId, jobId)` every second.
- Stop on terminal status, reset, or a new submission.
- On terminal, load the result or show the failure.
- On WebSocket reconnect, `session.ready` reconciles; stop polling if it adopts the same job.

Start polling from the connection-status handler as well as after submission, so a mid-run disconnect does not leave the UI stuck. Polling is a fallback, not the primary path. It also makes manual submission work without an active MCP agent.

### 10.8 Reload/reconnect restoration

On `session.ready`:

- Always apply MCP status.
- If `active_run` exists and matches `pendingClientRequestId` or the job stored in `sessionStorage`, adopt it, set `pageState = "running"`, and arm the timeout.
- If `last_job` exists and matches the stored job ID, adopt it:
  - `Success`: load the result.
  - `Failed`: show the failure.
  - `Pending`/`Processing`: treat as active.
- If neither matches, update only the history list. Do not auto-display another tab's result.

`resetPage` must:

- Clear the active job and pending client ID.
- Abort polling and result fetch.
- Clear `go-js-active-job-v1` from `sessionStorage`.
- Clear the existing result/highlight state as today.

### 10.9 MCP execute handling

`handleExecuteRequest`:

1. Reject as today if the editor is not ready or is busy.
2. Replace the editor contents.
3. Call `sendRun(code, message.request_id)`; the execute request ID doubles as `client_request_id` and is sent as `execute_request_id`.
4. If submission fails, send `execute.rejected` with the error. The backend ignores a late rejection if the pending request was already accepted.

Multiple tabs may call `sendRun` with the same execute request ID. The backend returns the same job; only the first tab attaches the MCP waiter. Each tab that initiated the submission may fetch the result; duplicate downloads across tabs are acceptable and bounded by the number of tabs.

### 10.10 History

Keep the existing history refresh triggers, but use the metadata-only endpoint. Refresh on `Pending` and on terminal status. Do not refresh on every `Processing` event.

The history list continues to load source into the editor; it does not fetch historical result bodies. That is unchanged.

## 11. Tests, validation, and acceptance

### 11.1 Backend tests

Update `web/backend/mcpserver/session_test.go` and add `jobs_test.go`.

Required tests:

1. **Submission**
   - `TestCreateJobReturnsAcceptedPending`
   - `TestCreateJobIsIdempotent`
   - `TestCreateJobSameKeyDifferentSourceConflicts`
   - `TestCreateJobRejectsBusySession`
   - `TestCreateJobRejectsOversizedCode`
   - `TestCreateJobRequiresClientRequestID`
2. **Status**
   - `TestJobStatusPending`
   - `TestJobStatusSuccessHasResultURL`
   - `TestJobStatusFailedHasError`
   - `TestJobStatusIsSessionScoped`
   - `TestJobStatusUnknownReturns404`
3. **Result**
   - `TestJobResultReturnsRawJSON`
   - `TestJobResultGzipWhenAccepted`
   - `TestJobResultIdentityWhenGzipNotAccepted`
   - `TestJobResultGzipQZero`
   - `TestJobResultLegacyBase64`
   - `TestJobResultPendingReturns202`
   - `TestJobResultFailedReturns409`
   - `TestJobResultIsSessionScoped`
   - `TestJobResultUnicodeRoundTrip`
   - `TestJobResultETag304`
4. **Protocol**
   - `TestMultipleTabsReceiveJobStatus`
   - `TestProcessingStatusIsPublished`
   - `TestReconnectReceivesLastJobMetadata`
   - `TestSessionReadyCarriesActiveRunMetadata`
   - `TestWorkerUpdateBeforeSessionBind` (manager routes by session ID; no job is lost)
   - `TestTerminalUpdateBeforeSubmitReturns` (no stale Pending overwrites a completed job)
5. **MCP**
   - `TestMCPExecuteFlowUsesHTTPSubmission`
   - `TestMCPExecuteSameRequestFromTwoTabsCreatesOneJob`
   - `TestMCPExecuteRejected`
   - `TestMCPExecuteOutputComesFromWorker` (no full-result decode)
6. **History/random**
   - Existing tests remain, but the fake store should assert that the metadata path does not expose `result`.

Update `fakeStore` to implement the new `JobStore` interface, including idempotency and metadata/result separation. Keep the fake in-memory; no PostgreSQL dependency is required for unit tests.

Run:

```sh
cd web/backend
go test ./...
go vet ./...
```

### 11.2 Frontend checks

```sh
cd web/frontend
npm install
npm run check
npm run build
```

There is no frontend unit-test harness. Add manual browser checks for:

- Manual submission over HTTP.
- Terminal success result fetch with gzip.
- Unicode output.
- Failure mode (syntax/runtime error) shows the stored message.
- Reload during `Processing` restores the active job and eventually the result.
- Reload after success restores the result from `sessionStorage`.
- WebSocket disconnect during execution; polling fallback completes the UI.
- Two tabs: both see status/history; only the submitting tab fetches the result.
- MCP execute with one tab.
- MCP execute with two tabs: one job, no duplicate execution.
- MCP rejection path (busy editor).
- Legacy historical job still opens its source and result.

### 11.3 End-to-end acceptance

1. Submit the Fibonacci example.
2. Confirm WebSocket messages contain no `result` field and are small (well under 1 KiB).
3. Confirm `GET .../result` with `Accept-Encoding: gzip` returns roughly 0.6-0.7 MiB for the measured Fibonacci capture, with `Content-Encoding: gzip`.
4. Confirm the decoded body is raw JSON, not a JSON string, and contains `format_version: 2`.
5. Confirm the animation renders and Unicode output is correct.
6. Confirm history/random responses never contain `result` and that SQL uses metadata columns.
7. Confirm `Pending` and `Processing` appear before terminal status.
8. Confirm a duplicate POST with the same `client_request_id` returns the same job.
9. Confirm an MCP `execute` call still returns output and job ID and does not decode the full trace.
10. Restart the backend during a queued/running job and verify:
    - pending rows are picked up by the worker poll;
    - stale processing rows are marked failed after the threshold;
    - the status endpoint still returns terminal rows;
    - the frontend can recover via polling/status.

### 11.4 Performance measurements to record

- WebSocket bytes per status event.
- Result bytes with and without gzip for Fibonacci, n-queens, game-of-life, and mandelbrot.
- Backend gzip CPU per result fetch.
- Database bytes read for history/random before and after.
- Browser time to parse the result and build the animation (expected to remain significant; this is the Phase 2 trigger).
- Peak browser heap for the Fibonacci animation (expected to remain significant).

Do not claim Phase 2 success from Phase 1 measurements.

## 12. Rollout and rollback

### 12.1 Rollout order

1. Add the migration.
2. Add `db` methods and `jobresult` package.
3. Update worker/manager/session and tests.
4. Add HTTP handlers and routes.
5. Update frontend.
6. Update README and the investigation status note.
7. Deploy backend and frontend together.

The frontend must not be deployed before the new routes exist. The backend must not be deployed before the new frontend exists, because it removes `run.submit`. Deploy both artifacts together, with the migration applied first.

### 12.2 Rollback

- New success rows remain base64, so the old backend's `run.result` path and the old frontend's `atob()` still work for jobs created during the new deployment.
- The new columns are additive; the old code ignores them.
- The down migration is destructive only for the new metadata; it can be applied after reverting the application if rollback is required.
- A job accepted by the new backend but never seen by the old frontend can be recovered by reloading after rollback because the old backend still replays `lastResult` from memory only if it was running during the job. After a restart, old history still lists the job but the old UI does not fetch single results. This is an acceptable documented limitation of rollback, not a production guarantee.

### 12.3 What must not be deployed

- Do not deploy the frontend result fetch without the result route.
- Do not deploy a backend that removes `run.submit` without the HTTP submission frontend.
- Do not deploy a schema where `result_encoding` is required without a default.

## 13. Phase 2 appendix: selective trace loading (deferred)

Phase 2 starts only after Phase 1 is merged and measured. Its trigger is a remaining user-visible problem in first-useful-result time, animation start time, seek latency, or browser heap/long tasks.

### 13.1 Artifact model

At completion, the worker already has the decoded JSON. Phase 2 adds a split step:

- **Overview artifact:** `format_version`, `output`, `debug_chunks`, `ast`, execution metadata (`schema_version`, `status`, `truncated`, `total_frames`, `dropped_frames`, `duration_ms`, `limits`), and a trace manifest.
- **Trace chunks:** groups of frames with a checkpoint:
  ```json
  {
    "schema_version": 1,
    "chunk_index": 0,
    "start_index": 0,
    "end_index": 499,
    "start_vm": null,
    "start_output_length": 0,
    "frames": []
  }
  ```
  A chunk must carry the last VM state and cumulative output length at its start so it can render independently. `frame.seq` can have gaps after capture truncation; use stored array indices for seeking, not `seq`.
- **Storage:** add a `job_artifacts` table with `(job_id, kind, chunk_index, content, content_encoding, byte_size, created_at)` and a unique key. Store overview and each chunk as independently compressed bytes or as text with explicit encoding. Keep the full `result` column for legacy jobs.
- **Manifest:** status/overview includes `result_mode: "split"`, `overview_url`, `trace_chunk_url_template`, `chunk_count`, `total_frames`, and `truncated`. Do not advertise a chunk URL before the chunk row is committed.

### 13.2 API

- `GET /session/{s}/jobs/{j}/overview` -> overview JSON.
- `GET /session/{s}/jobs/{j}/trace/chunks/{n}` -> one chunk JSON.
- `GET /session/{s}/jobs/{j}/result` for a split job returns `409 result_is_split` with the overview/manifest URL; it never silently omits frames.
- Legacy/full jobs continue to use `/result`.

### 13.3 Frontend

- Load the overview when the result is selected; render output, AST, and bytecode immediately.
- Load the first trace chunk only when the Animation tab is opened.
- Keep a bounded decoded-frame window (for example, three chunks or 1,500 frames). Evict old chunks; do not concatenate everything into the current giant array.
- Seeking fetches the chunk containing the target global index and reconstructs VM/output from that chunk's checkpoint.
- Prefetch the next chunk near the end of the current window.
- A chunk fetch failure shows a retry state; it is not reported as a truncated runtime trace.

### 13.4 Measurements and acceptance

- First usable output/AST without any frame body transfer.
- Animation opens after downloading only the overview plus one chunk.
- Seeking across chunk boundaries works.
- Peak browser heap is bounded by the window.
- Total compressed bytes remain close to the single-document gzip size (the investigation's 500-frame exploratory split was about 666.6 KiB versus 662.3 KiB).
- Legacy full results still render exactly as before.

Phase 2 must not be folded into Phase 1. It changes the storage contract and the animation data flow, and it deserves its own review and measurements.

## 14. Explicit non-goals and guardrails

The implementation agent must not:

- Add SSE or `EventSource` in Phase 1.
- Add Brotli, MessagePack, CBOR, protobuf, or FlatBuffers in Phase 1.
- Change the runtime's structured output format or remove base64 from the runtime in Phase 1.
- Send a result body over WebSocket.
- Query the `result` column from history, random, or status handlers.
- Call `atob()` or parse a base64 string in the frontend.
- Add direct backend queueing for MCP executions.
- Add agent leases, heartbeat endpoints, or presence guarantees.
- Return a partial `InterpretDetails` object from `/result`.
- Use `run.accepted`, `run.result`, `run.error`, or `run.submit` after the new protocol lands.
- Implement multi-replica fan-out or authentication as part of this change.

## 15. Definition of done

Phase 1 is done when all of the following are true:

- [x] Migration `000003` applies cleanly to a database containing legacy rows.
- [x] `go test ./...` in `web/backend` passes.
- [x] `go vet ./...` in `web/backend` passes.
- [x] `npm run check` and `npm run build` in `web/frontend` pass.
- [x] `./test.sh` passes (runtime regression suite).
- [ ] A manual Fibonacci run returns raw gzipped JSON over HTTP, with no base64 in the browser path.
- [x] WebSocket traffic contains only small status/metadata messages.
- [x] History and random-script queries do not select `result`.
- [x] `Pending`, `Processing`, `Success`, and `Failed` are all observable in the UI.
- [x] Idempotent submission and same-key/different-source conflict are covered by tests.
- [x] MCP execute works with one and two tabs and returns output without decoding the full trace.
- [ ] Reload during processing and after success recovers from the status/result endpoints.
- [x] Legacy success and failure rows remain readable.
- [x] `README.md` and the investigation status note point to the new flow and this plan.
