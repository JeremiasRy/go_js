# Event loop and Promise implementation plan

**Status:** planning only; this document does not change runtime behavior.

## Goal

Fix the Promise and asynchronous-scheduling failures as one runtime problem. The
target is that async functions, manually constructed promises, `.then()` chains,
Promise combinators, timers, and callback-driven workers all use the same
settlement and scheduling rules. Do not make the lab pass by rewriting its
application code or adding special cases for a particular callback shape.

## Baseline and evidence

`README.md` records observations from `go_js.execute` on 2026-11-14, but that
runtime did not expose its build or commit. This checkout contains later async
fixes, so those observations are useful reproductions, not a reliable statement
of the current branch's behavior.

A local binary built from this checkout was run against the non-network probes
on 2026-11-14. Most of the old failures now pass locally: probes 06-15, 18-23,
29, and 31 passed, as did callback probes 25, 26, and 28. The current local
failures observed were:

| Probe | Current checkout observation | Node 24 expectation |
|---|---|---|
| 16, custom `all()` over async functions | Timed out after 5 seconds | `custom all async: a,b,c` |
| 17, `mapLimit()` over async functions | Exited with `mapLimit: ,,,` | `mapLimit: 10,20,30,40` |
| 24, async work wrapped in a manual promise and raced | One normal run exited successfully with no output; a debug run exited successfully and printed the expected line | `wrapper race: a` |

These observations are intentionally marked as symptoms, not root causes. In
particular, local probe 07 (`Promise.all` over async functions) passes, despite
the remote report saying it fails. Network probes and the full live-endpoint
lab were not used for this local baseline. Re-run them against a pinned runtime
before treating any remote-only finding as an active regression.

## Current design and likely fault boundaries

- `runtime/queue/queue.go` keeps package-global FIFO task and microtask queues.
  `VM.Run` drains microtasks before tasks, then polls for more work.
- `runtime/eventloop/eventloop.go` is also package-global. It starts worker
  goroutines for dispatched jobs and forwards their callbacks to the VM queue.
  `runtime/native/constructor.go` uses a blocking timer job; async-function
  `Work` uses a blocking promise wait.
- `runtime/native/promise.go` stores promise state, reactions, observers, and a
  one-slot notification channel. `runtime/vm/vm.go` has a separate resolution
  procedure, reaction dispatch, and async-function call paths.
- Await currently has its own thenable conversion path in `OP_AWAIT`; a
  pending await dispatches an async-function job that blocks in
  `ObjPromise.Listen`. The worker then writes to the saved async-function state.
- Async calls can enter through bytecode `OP_CALL` or `callValue`. They do not
  share one start/return path, and the compiler's `ReturnsPromise` annotation is
  also used to choose runtime behavior.
- Resolver objects, promise reactions, async activations, and queued callbacks
  cross the custom heap/GC boundary. In particular, resolve/reject wrappers
  retain a raw promise pointer while their `GetReferencingValues` methods
  currently return no roots. `promiseReaction` also assumes its `Derived` value
  is an `ObjPromise` without checking the type.

This makes the reported symptoms consistent with several interacting risks:
inconsistent async-call paths, worker-side mutation of async state, a
single-consumer wait channel, mismatched Promise resolution rules, callback
argument/closure handling, missing GC roots, and event-loop idle/close races.
The plan validates these independently instead of assuming one explains every
failure.

## Runtime invariants to implement

1. **One owner for JavaScript state.** One runtime/event-loop thread owns the VM,
   heap, promise state transitions, callback invocation, and async continuations.
   Timer or I/O workers may wait or perform host work, but may only post a
   completion record; they must not run JS, settle a promise, or mutate a saved
   VM stack.
2. **Explicit job classes and ordering.** Promise reactions, `await` resumptions,
   and `queueMicrotask` callbacks enter the FIFO microtask queue. Timers and
   other host callbacks enter the task queue. Finish the current JS turn, then
   drain microtasks to exhaustion before selecting the next task. A settled
   promise still schedules its reaction; settlement never invokes user handlers
   inline.
3. **One Promise state machine.** A promise settles at most once. Every
   fulfillment path—including executor `resolve`, async returns, combinators,
   and thenable assimilation—uses the same resolution procedure. Reject and
   resolve functions share a single once-only resolving capability.
4. **One async-function contract.** Every async-function call immediately
   returns a promise. Its return value is adopted through the normal resolution
   procedure, including a returned promise or thenable. Awaiting any value uses
   the same promise-normalization and continuation path.
5. **Deferred work is rooted.** Every promise, resolver pair, reaction handler,
   derived promise, saved activation, argument, and callback retained across a
   task boundary is visible to the custom GC until consumed or cancelled.
6. **Idle and close are explicit.** Queued jobs, active timers/host operations,
   and the scheduler's closed state are accounted for atomically. A pending
   promise by itself is not an event-loop handle; an active timer or host
   operation is. Closing must not race with posting a completion or rescheduling
   an interval.

## Staged implementation

### 0. Pin the baseline and add deterministic regression cases

- Keep the remote MCP findings and the local checkout baseline distinct. Record
  the runtime revision used for every new result.
- Add deterministic versions of probes 16, 17, and 24 to the normal test suite
  with Node 24 expected output. Keep network calls and randomized failures out
  of these tests.
- Add focused cases for two async functions awaiting the same pending promise,
  resolver/rejecter calls through nested helpers and timers, late `.then()` on a
  settled promise, and a callback retained across forced GC.
- Preserve the passing cases as regression gates: especially 07-15, 18-23,
  25-26, 28-31 and the existing `test/test-scripts/promise-*.js` suite.
- Capture microtask/task ordering with a small trace: synchronous output,
  promise reaction, nested reaction, timer callback, and nested timer.

**Exit gate:** each selected script has a deterministic Node 24 result and a
reproducible result on this checkout; no network or random behavior is needed to
exercise the failing contracts.

### 1. Give the scheduler runtime ownership and a single completion path

- Introduce a scheduler/event-loop instance owned by one runtime instead of
  package-global `queue.q` and `eventloop.el`. Pass it to the VM and timer/host
  scheduling APIs. Keep the existing FIFO queue implementation initially if
  that reduces risk; the ownership change is more important than a wholesale
  queue rewrite.
- Make the VM the only consumer that invokes JS callbacks. A background job
  returns a completion record to the scheduler; the VM turns it into a rooted
  task or microtask callback.
- Remove Promise waiting from `object.Job.Work`. Promise settlement will notify
  registered reactions, not wake a goroutine blocked on `Listen`.
- Replace ticker-based idle polling with a coalesced wake signal plus an
  explicit active-source count. Queue state, active timers/host work, and close
  state must be checked under one scheduler lifecycle protocol so a late post
  cannot be lost.
- Preserve the timer API and cancellation behavior first. Once ordering and
  liveness tests pass, move timers to a scheduler-owned deadline/sequence queue
  if the current per-timer blocking jobs still complicate accounting.

**Exit gate:** nested timers, concurrent timers, cancellation, and a timer
scheduled by a callback drain exactly once; the loop neither exits early nor
hangs after all active sources and queues are empty. No worker executes JS or
touches VM/heap state.

### 2. Centralize Promise settlement and thenable assimilation

- Make `runtime/vm`'s resolution procedure the sole path for state changes.
  Audit direct `ObjPromise.Resolve`/`Reject` calls in constructors, async
  returns, `OP_AWAIT`, and host APIs; keep low-level state mutation private to
  the settlement implementation.
- Replace the promise's one-slot `c`/`Listen` protocol with pending state plus
  registered reactions/observers. A promise can have any number of awaiters and
  observers; settlement schedules each once.
- Use one resolver capability shared by resolve and reject. The first call
  wins, including when resolution starts asynchronous thenable assimilation.
  Resolve with a promise/thenable adopts it; reject after a winning resolve is
  ignored. Reject self-resolution and chaining cycles.
- Retrieve a thenable's `then` once, convert getter exceptions to rejection,
  invoke callable `then` in a queued job with the original receiver, and honor
  only its first resolve/reject/throw outcome.
- Store cross-task promise references as GC-visible handles/roots, not only raw
  Go pointers. Add an audit for `Promise`, resolver functions, observers,
  reactions, and async activations; release callback roots when jobs are
  consumed or cancelled.

**Exit gate:** settlement-once, nested thenables, throwing `then` getters,
promise adoption, self-resolution, and multiple listeners pass deterministic
tests without panics or lost callbacks.

### 3. Route async functions and `await` through that Promise machinery

- Create an activation record per async call. Keep function metadata separate
  from mutable paused execution state; do not let shallow callable clones share
  a continuation accidentally.
- Use one async-call entry point for bytecode calls and `callValue`/callback
  calls. It creates and returns the result promise before running the body.
  Normal returns resolve/adopt through the shared procedure; thrown values
  reject it.
- Remove `ReturnsPromise` as a semantic shortcut. A compiler hint may remain for
  optimization only after tests prove it cannot change identity or settlement
  semantics.
- In `OP_AWAIT`, normalize the operand using the same resolution procedure.
  Save the activation, register fulfillment/rejection reactions, and enqueue a
  resume microtask. Fulfillment pushes the value; rejection resumes by throwing
  into the saved async frame so `try/catch` behaves normally. Already-settled
  operands still resume asynchronously.
- Do not dispatch an async function as a blocking event-loop job. This removes
  the single-consumer channel issue and keeps stack/exception state on the VM
  owner thread.

**Exit gate:** direct, stored, array-element, sequential, and concurrent awaits
all work; two awaiters of one pending promise both resume; async functions that
return manual promises adopt them; rejection is catchable and does not strand
the loop.

### 4. Make reactions, combinators, and callback invocation share the contract

- Keep `.then()`/`.catch()`/`.finally()` on one typed reaction path. `.then()`
  always returns a fresh derived promise; absent/non-callable handlers pass
  through the outcome; handler returns are adopted; throws reject. `finally`
  waits for its return value and preserves the prior outcome unless the
  finalizer fails.
- Ensure reaction records retain their derived promise safely. Validate the
  derived value before use so an invariant violation reports a controlled
  runtime error instead of an interface-conversion panic.
- Implement `all`, `allSettled`, `race`, and `any` on the same per-input
  normalization/reaction API. Keep input order, settle each slot once, preserve
  early-settlement behavior, and cover empty iterables and iterator failures.
- Verify native resolver/rejecter functions and user functions go through the
  same call protocol. Preserve callback arguments, `this`, closure values, and
  return values through `setTimeout`, helpers, `.then`, and combinators.

**Exit gate:** the native combinator tests and hand-written `all()`/`mapLimit()`
tests agree; no result slots are left empty and no race winner is overwritten
by a later result.

### 5. Exercise the task-queue workload and close the loop

- Add an in-repository deterministic worker-pool smoke test using short timers
  and local results. Assert `maxActive <= 3`, ordered fan-out results, one
  terminal outcome per task, timeout winner/late loser behavior, retries, and
  eventual drain. Do not assert exact timing.
- Keep `lab/task-queue-lab.js` as the high-level reference workload, not a
  routine CI test: it uses live endpoints, randomness, and explicitly does not
  cancel timed-out work. Compare its invariants and event timeline with Node 24
  when running it manually.
- Run the full Go/runtime tests and JIT/interpreter differential checks after
  each stage. Use execution tracing to check actual queue order and active-work
  accounting when a test fails.

## Scope boundaries and acceptance

- The `for (;;)` compiler crash reported by probe 04 is independent of Promise
  settlement. Track it separately; use the lab's `while (true)` form while
  validating the scheduler and Promise work.
- `AbortController` and cancellation of losing `Promise.race` inputs are not
  prerequisites. A timeout may win while the underlying operation continues;
  the race result must remain settled once and the late completion must be safe.
- Live `fetch` performance and network fan-out are separate host-I/O work. Any
  future I/O worker must post a completion to the VM owner rather than touch the
  heap or settle a promise itself.
- Definition of done: probes 16, 17, and 24 print their Node 24 expected output
  and terminate; the current passing probes and all existing
  `test/test-scripts/promise-*.js` tests remain green; repeated runs do not hang,
  lose callbacks, or panic; and the deterministic worker-pool smoke test drains.

Validation commands for implementation stages:

```sh
(cd runtime && go test ./...)
./test.sh
GO_JS_DIFFERENTIAL=1 ./test.sh
(cd runtime && go test -race ./...)
```

Run the race detector after the cross-goroutine state ownership changes; use the
full Docker suite and JIT differential run as final regression gates.
