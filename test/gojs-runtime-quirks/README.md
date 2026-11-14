# go_js runtime quirks

Reproduction probes for the JavaScript runtime behind the `go_js.execute` MCP
tool, collected while trying to run a task-queue-heavy workload.

**Observed:** 2026-11-14
**Baseline:** Node.js 24 (all snippets are valid, runnable ES modules)
**Runtime under test:** `go_js.execute` (Go-based JS VM; version/commit not
exposed by the MCP interface)

**Implementation plan:** [event loop and Promise plan](IMPLEMENTATION_PLAN.md)

The short version: timers, `fetch`, classes, closures and manual promises work
in go_js, but the Promise implementation breaks down as soon as async-function
promises or concurrent orchestration are involved. Some failures are crashes,
some are silent no-ops, and at least one is an outright wrong resolved value.

---

## Layout

```
.
├── package.json              # ESM ("type": "module"), Node >= 24
├── README.md
├── lab/
│   └── task-queue-lab.js     # full 17-task / 3-worker queue lab (Node baseline)
└── probes/
    ├── 01-runtime-capabilities.js
    ├── ...
    ├── 31-async-returns-manual-promise.js
    └── run-all.js            # sequential probe runner
```

Each probe starts with a comment block stating:

- what it checks,
- the Node 24 baseline behavior,
- what go_js actually did on 2026-11-14.

## Quick start

```bash
# Node 24 is the baseline. Either use a Node 24 install, or:
npx -y node@24 --version        # v24.x

# Run all probes under Node 24:
npx -y node@24 probes/run-all.js

# Run the full queue lab:
npx -y node@24 lab/task-queue-lab.js

# Or via npm scripts:
npm run test:node24
npm run lab:node24
```

To reproduce a probe in go_js, paste the file contents into:

```js
go_js.execute({ code: <file contents> });
```

The probes are intentionally small and independent. Network probes use
`jsonplaceholder.typicode.com` and `httpbin.org`.

---

## Results overview

Legend:

- ✅ same behavior as Node 24
- ⚠️ exits successfully but the behavior is wrong, missing, or silent
- ❌ throws, crashes, or hangs in go_js

| Probe | Checks | Node 24 | go_js observed |
|---|---|---|---|
| 01 | capabilities: timers, fetch, AbortController | ✅ | ⚠️ `AbortController` is `undefined` |
| 02 | plain fetch (three requests) | ✅ | ✅ all HTTP 200 |
| 03 | class getters + async methods | ✅ | ✅ |
| 04 | `for (;;)` | ✅ | ❌ compiler SIGSEGV |
| 05 | bitshift, `for...of`, class loops, getters, `undefined` | ✅ | ✅ |
| 06 | `Promise.all` with plain values | ✅ | ✅ |
| 07 | `Promise.all` over async-function promises | ✅ | ❌ `TypeError: Value is not callable` |
| 08 | `.then()` on an async-function promise | ✅ | ⚠️ callback never runs |
| 09 | `Promise.race` between manual promises | ✅ | ✅ |
| 10 | `Promise.race` where the async promise wins | ✅ | ❌ `TypeError: Value is not callable` |
| 11 | `Promise.race` where the timeout wins | ✅ | ✅ |
| 12 | `await` async promise stored in a variable | ✅ | ❌ `TypeError: Value is not callable` |
| 13 | `await` async promise stored in an array | ✅ | ❌ `TypeError: Value is not callable` |
| 14 | two sequential awaits of async calls | ✅ | ❌ `TypeError: Value is not callable` |
| 15 | hand-rolled `all([])` | ✅ | ❌ `TypeError: Value is not callable` |
| 16 | hand-rolled `all()` over async promises | ✅ | ❌ `TypeError: Value is not callable` |
| 17 | `mapLimit` bounded concurrency | ✅ | ❌ `TypeError: Value is not callable` |
| 18 | manual promises + completion counter | ✅ | ✅ both patterns passed separately |
| 19 | resolve wrapper called directly from a timer | ✅ | ✅ |
| 20 | resolve wrapper passed as a callback parameter | ✅ | ⚠️ resolves with `[object Function]` |
| 21 | promise resolution via helper callback | ✅ | ⚠️ exits 0 with no output |
| 22 | reject with error via helper callback | ✅ | ⚠️ catches `Value is not callable` instead of the error |
| 23 | error callback logs, then resolves a literal | ✅ | ✅ |
| 24 | async work wrapped in a manual promise, raced | ✅ | ⚠️ exits 0 with no output |
| 25 | callback-only 3-worker pool | ✅ | ❌ `TypeError: Value is not callable` |
| 26 | callback-only timeout wrapper | ✅ | ❌ `TypeError: Value is not callable` |
| 27 | callback fan-out with `fetch` | ✅ | ❌ 10 s tool timeout, no output |
| 28 | callback-only retry with backoff | ✅ | ❌ `TypeError: Value is not callable` |
| 29 | nested timers resolving a manual promise | ✅ | ✅ |
| 30 | `fetch().then()` chains | ✅ | ✅ |
| 31 | async function returning a manual promise | ✅ | ❌ `TypeError: Value is not callable` |

Two caveats:

1. The go_js results were not perfectly deterministic. Near-identical variants
   sometimes produced a hard error in one run and empty output with exit 0 in
   another. The table records the failure mode seen for each original probe.
2. The files here are cleaned-up versions of those probes; a rerun may produce
   a different member of the same failure family.

---

## Findings

### 1. Basic runtime works, but there is no `AbortController`

`setTimeout`, `setInterval`, `queueMicrotask`, `performance` and `fetch` are
present. `AbortController` is not. That matters because the obvious way to add
a fetch timeout is:

```js
const c = new AbortController();
setTimeout(() => c.abort(), 1000);
await fetch(url, { signal: c.signal });
```

In go_js this fails at `new AbortController()` with a `ReferenceError`.
Timeout handling therefore has to fall back to `Promise.race`, which runs into
the problems below.

Probe: `01-runtime-capabilities.js`.

### 2. `for (;;)` crashes the compiler

This is the only compiler-level crash found. It happens before any user code
executes:

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV ...]
go_js/compiler.withAstTracking(...)
go_js/compiler.generateByteCode(...)
...
main.main() /app/main.go:116
```

Every other control-flow shape tested (plain `for`, `while`, `for...of`,
comparisons, class methods) compiled fine. Replacing `for (;;)` with
`while (true)` gets past the compiler — it is even the only source change
needed to reach the *next* failure in the queue lab.

Probes: `04-for-ever.js`, `05-control-syntax.js`.

### 3. Promise support is a minefield

The working subset is narrow:

| Pattern | go_js |
|---|---|
| `new Promise` resolved by a timer | ✅ |
| sequential `await` of those manual promises | ✅ |
| `Promise.all` of plain values | ✅ |
| `Promise.race` of manual promises | ✅ |
| `Promise.race` where the timeout rejects first | ✅ |
| nested timers resolving a manual promise | ✅ |
| `fetch().then()` chains | ✅ |
| `Promise.all` of async-function promises | ❌ |
| `.then()` on an async-function promise | ⚠️ silent no-op |
| store + `await` an async-function promise | ❌ |
| `await` two async calls in a row | ❌ |
| `await` an async function that returns a manual promise | ❌ |
| `Promise.race` where the async promise wins | ❌ |
| async work wrapped inside a manual promise | ⚠️ silent |
| `mapLimit` / worker orchestration | ❌ |

The dominant error is:

```
runtime error: TypeError: Value is not callable
```

A notable one-off: passing a resolve-wrapper through a callback parameter
resolved the awaited promise with the **function itself**, producing
`B=[object Function]` instead of `B=y` (probe 20).

Probes: `06`–`17`, `19`–`24`, `31`.

### 4. Callback-only code is not a safe escape hatch

After the Promise failures, a purely callback-driven queue was attempted:
worker pool, timeout wrapper, retry loop and fetch fan-out, all using
`setTimeout` and completion counters, no promises.

- Worker pool: `TypeError: Value is not callable` (probe 25)
- Timeout wrapper: same error (probe 26)
- Retry loop: same error (probe 28)
- Fetch fan-out: hung until the 10 s tool timeout (probe 27)

Some callback shapes do work: a completion counter driven by timers
(probe 18), a resolve-wrapper called directly from a timer (probe 19), error
logging followed by resolving a literal (probe 23), and nested timers
(probe 29). But there is no combination that reliably runs a concurrent
queue.

### 5. The full queue lab never ran in go_js

`lab/task-queue-lab.js` is a 17-task workload with:

- an aging priority queue over a binary min-heap,
- 3 long-lived workers,
- a token-bucket rate limiter (4 req/s),
- exponential backoff + jitter retries,
- a circuit breaker (5 failures → open 400 ms → half-open probe),
- `Promise.race` timeouts,
- live `fetch` calls, a nested `mapLimit(2)` fan-out,
- a dead-letter queue and ASCII timeline.

Under Node 24 it completes in roughly 3–4 s, e.g.:

```json
"totals": {
  "enqueued": 17,
  "dispatched": 17,
  "completed": 13,
  "deadLettered": 4,
  "retries": 13,
  "timeoutEvents": 2,
  "recoveredAfterRetry": 1
},
"elapsedMs": 3386,
"throughputPerSec": 3.84
```

(Counts vary with the random flaky-service failures.)

In go_js:

1. Original version (with `for(;;)` in the heap and worker loops):
   compiler SIGSEGV, identical to probe 04.
2. After replacing those with `while (true)`: VM panic at runtime in the
   Promise reaction machinery:

   ```
   panic: interface conversion: object.Object is *native.RejectFunc,
          not *native.ObjPromise
   go_js/vm.(*VM).promiseReaction(...)
   go_js/vm.(*VM).settlePromise.func2()
   go_js/vm.(*VM).Run(...)
   ```

   The likely trigger is the `Promise.all([worker('w1'), worker('w2'),
   worker('w3')])` barrier, which is exactly the pattern isolated by probes
   07 and 17.

The lab is included as the Node-24 reference workload, not as a working go_js
example.

---

## Error signatures seen

| Signature | Where |
|---|---|
| `panic: runtime error: invalid memory address or nil pointer dereference` in `compiler.withAstTracking` | `for (;;)` compile |
| `TypeError: Value is not callable` | async promises, combinators, callback pool/timeout/retry |
| `panic: interface conversion: object.Object is *native.RejectFunc, not *native.ObjPromise` | full lab, `Promise.all` worker barrier |
| exit 0 with empty output | promises resolved through helper callbacks, async wrappers |
| `[object Function]` as a resolved value | resolve-wrapper passed as a callback |
| tool timeout after 10 s with no output | callback-style `fetch` fan-out |

---

## If you must run JS in go_js

- Replace `for (;;)` with `while (true)`.
- Build timeouts from manual promises; do not rely on `AbortController`.
- Keep async/await to a single direct call if possible. Avoid storing async
  promises, arrays of them, `Promise.all`, `.then` on them, and races they can
  win.
- `await fetch(url)` for a single request and `fetch().then()` chains launched
  inside one manually constructed promise are the safest network patterns
  (probes 02 and 30).
- Manual-promise timers and completion counters are the safest scheduling
  primitives (probes 18 and 29).
- For a real concurrent task queue, run it under Node 24 and treat go_js as a
  scripting sandbox, not a scheduler.

## Reproducing the baseline

Local validation used `npx -y node@24` (`v24.22.3`). The default `node` on the
machine that produced this folder was `v20.20.2`; the package declares
`"engines": { "node": ">=24" }`. The code also runs on Node 18+ in practice
(global `fetch` is the only recent requirement), but Node 24 is the stated
baseline.
