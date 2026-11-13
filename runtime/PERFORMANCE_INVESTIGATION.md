# Runtime Performance Investigation: Tracer vs. VM/Bytecode

**Status:** peer-reviewed investigation only; no production changes in this
branch yet<br>
**Branch:** `perf/tracing-vs-vm-investigation`<br>
**Base commit:** `bda45c4` (`feat/execution-animation-frontend`)<br>
**Date:** 2026-11-13<br>
**Environment:** Linux amd64, Go 1.26.3

## TL;DR

The tracer is the dominant problem. With the web demo's current flags, tracing
makes the non-JIT-heavy examples **15–140× slower** than the same run without
tracing. A prototype that stops taking step snapshots after the frame budget has
already been exhausted brings every web example comfortably under the 10-second
deadline:

| Web example | No trace (`-jit=auto`) | Current trace | Trace + early-out prototype |
| --- | ---: | ---: | ---: |
| Mandelbrot | 0.20 s | 25.70 s | **0.49 s** |
| Game of Life | 1.52 s | 28.84 s | **1.90 s** |
| Eight queens | 0.54 s | 7.92 s | **1.03 s** |
| Fibonacci | 3.20 s | 3.98 s | **3.50 s** |

The prototype retained the same number and kinds of stored frames, and nearly
the same payload, but it did **not** preserve trace semantics. It returned
before the normal per-step event flush and VM-state refresh, and it stopped
counting skipped steps. As a result,
`total_frames`, `dropped_frames`, and `seq` changed, event frames could carry
stale VM state, and output deltas could be attached to an earlier event.

A production version should still skip snapshot construction for rejected step
frames, but it must keep event flushing, event-state refresh, output capture,
and truncation accounting correct. The timing argument for doing this first is
unchanged; the implementation is small but not a one-line guard.

**Recommendation:** prioritize tracing work first. The first change should be
budget-aware capture with the trace side effects and accounting preserved,
followed by removing per-step locks/allocations and making value snapshots
cheaper. Re-profile after that change before choosing the next tracer
optimization. Bytecode/VM work is still useful, but it cannot plausibly recover
the 15–140× gap on its own, and the current JIT is inconsistent on exactly the
examples that are failing.

## 1. Problem setup

The web backend runs every submission as:

```text
go_js --structured --trace-execution --trace-max-frames=10000 /workspace/script.js
```

with a hard `EXECUTION_TIMEOUT = 10 * time.Second` in
`web/backend/microvm/microvm.go`. The four examples shipped by the frontend are
`mandelbrot.js`, `game-of-life.js`, `n-queens.js`, and `fibonacci.js` in
`web/frontend/src/lib/examples/`.

The deadline is currently violated by the workloads that spend most of their
time in interpreted bytecode. Fibonacci is different: its hot recursive function
is JIT-compiled, and JIT execution bypasses per-instruction `traceStep` calls, so
its trace overhead is much smaller.

## 2. Methodology and caveats

- Built the runtime from the base commit with `go build`.
- Timed the **whole process** around the built binary with Python
  `time.perf_counter()`; stdout was discarded for timing runs.
- The table above uses the median of three runs.
- All runs used the web flags (`--structured --trace-execution
  --trace-max-frames=10000`) and the default `-jit=auto` unless noted.
- These are host timings, not Firecracker/microVM timings. The microVM is likely
  slower, so the headroom shown by a fix should be treated as an upper bound.
- The early-out prototype was built in a throwaway copy of `runtime/`; it is not
  part of this branch. Its only change was to stop calling `traceStep` once a
  step frame had been dropped by the frame budget. That is a valid timing
  prototype, not a faithful trace implementation: it also skips queue-event
  flushing, `LastVM` refreshes, and output capture at the next instruction.
- The peer review reran the timings and compared the retained traces. Stored
  frame counts and payload sizes reproduced, but truncation metadata and event
  semantics did not. See Section 10 for the verification results and the small
  `queueMicrotask` reproduction.

## 3. Measurements

### 3.1 End-to-end times

Medians of three runs, seconds:

| Workload | JIT off, no trace | JIT auto, no trace | Current trace, JIT auto | Early-out prototype, JIT auto |
| --- | ---: | ---: | ---: | ---: |
| `mandelbrot.js` | 0.156 | 0.202 | 25.699 | 0.487 |
| `game-of-life.js` | 0.643 | 1.519 | 28.838 | 1.896 |
| `n-queens.js` | 0.070 | 0.543 | 7.919 | 1.031 |
| `fibonacci.js` | >120 | 3.197 | 3.981 | 3.501 |

Two observations matter:

1. Relative to the no-trace/JIT-auto baseline, current tracing is roughly
   **127×** slower for Mandelbrot, **19×** for Game of Life, **15×** for
   eight queens, and **1.25×** for Fibonacci.
2. Even a hypothetical **2× reduction in whole traced runtime** would leave
   Mandelbrot and Game of Life above 10 seconds on this host. A VM-only fix would
   need more than a 2.5–3× improvement just to break even, before microVM
   slowdown and safety margin. That is optimistic for interpreter work, because
   snapshot construction is largely independent of opcode dispatch.

### 3.2 The frame limit does not limit the work

Changing `--trace-max-frames` from 100 to 1000 to 10000 barely changes the
current trace time (single representative runs):

| Workload | Current trace, 100 frames | Current trace, 1000 frames | Current trace, 10000 frames |
| --- | ---: | ---: | ---: |
| Mandelbrot | 26.8 s | 27.1 s | 25.0 s |
| Game of Life | 31.4 s | 29.4 s | 27.4 s |
| Eight queens | 9.5 s | 8.3 s | 7.8 s |

The reason is visible in the produced traces. The current recorder still builds
a full snapshot for every instruction and only discovers at append time that the
frame should be dropped:

| Workload | Stored frames | Attempted frames | Dropped frames | Payload (base64 JSON) |
| --- | ---: | ---: | ---: | ---: |
| Mandelbrot | 9,001 | 2,522,077 | 2,513,076 | 39.0 MiB |
| Game of Life | 10,000 | 2,739,065 | 2,729,065 | 43.7 MiB |
| Eight queens | 10,000 | 715,179 | 705,179 | 48.9 MiB |
| Fibonacci | 9,063 | 45,327 | 36,264 | 61.3 MiB |

In other words, Mandelbrot spends most of its 25 seconds producing roughly
2.5 million snapshots that it immediately throws away.

The saved prototype kept the same stored step/event/JIT counts and near-identical
payloads on these examples, but its `total_frames`, `dropped_frames`, and `seq`
values changed because skipped steps were no longer appended or counted. Retained
frame counts alone therefore do not establish equivalent traces; Section 10
records the metadata and event-state differences.

A smaller full trace shows the per-frame cost/payload directly: a 500-iteration
loop produced **8,028 frames and 34.6 MiB** of base64 JSON in 0.30 s, while the
same script without tracing took 0.011 s. That is about **4.4 KiB per stored
frame** and a **27×** slowdown even when nothing is truncated.

### 3.3 Where the trace CPU goes

A CPU profile of the current Mandelbrot trace (`-jit=auto`) showed:

- `traceStep` / `snapshotVM`: dominant cumulative cost, about **57% / 54%** of
  sampled CPU.
- `valueView` / `valueViewDepth`: about **34%**, including number formatting,
  string quoting, heap lookups, and allocations.
- `trace.CurrentLimits`: about **16%**, mostly repeated recorder mutex
  acquisition from inside `snapshotVM` and `valueView`.
- GC mark/sweep work: about **38%** of samples, driven by the allocation volume
  of snapshots and value views.
- `runtime.growslice`: about **10%**, from repeatedly growing frame/stack view
  slices.

These numbers overlap because they are cumulative, but the message is clear:
the hot path is dominated by snapshot construction, repeated lock acquisition,
value rendering, and the resulting GC pressure.

## 4. Root causes in the current tracer

1. **A full VM snapshot is built before every interpreted instruction.**
   `traceStep` snapshots every call frame and up to 64 stack values. That is
   expensive even when the instruction is a simple local load or jump.
2. **The recorder/lookup path takes locks repeatedly per instruction.**
   `traceStep` -> `flushTraceEvents` -> `TakeQueueEvents` (mutex),
   `queue.Snapshot` (global trace lock + queue mutex), `loopState`
   (`CurrentLimits`, event-loop lock, timer registry lock, current-job lock,
   booted lock), `snapshotVM` (`CurrentLimits` again), then `SetLastVM`,
   `LastOutputLen`, and `AppendFrame` each lock again.
3. **Value rendering is general-purpose and allocation-heavy.**
   Numbers go through `native.String`, strings go through `strconv.Quote`, the
   result is converted to `[]rune` for truncation, and object values perform
   heap lookups. Every stack slot pays this cost every instruction.
4. **Dropped frames are fully constructed first.**
   The recorder enforces limits only after the expensive snapshot has been
   built. The limit therefore controls memory, not CPU.
5. **The trace payload is large and repetitive.**
   At ~4–6 KiB per frame and up to 10,000 frames, the structured response is
   tens of MiB. Queue/timer/event-loop state is usually unchanged between
   adjacent instructions but is repeated in full.
6. **JIT changes the trace profile.**
   JIT code does not emit per-instruction step frames, so Fibonacci is not as
   badly affected. This also means making more code JIT-compiled would make
   tracing faster partly by making the trace less detailed, not by making the
   tracer itself cheaper.
7. **`traceStep` has side effects beyond snapshot construction.**
   It flushes pending queue events, updates the VM state used by later event
   frames, and provides the next capture point for output deltas. Skipping the
   whole function changes trace semantics; an early-out can safely skip only the
   step snapshot and must preserve the rest.

## 5. Option A: bytecode/VM execution optimizations

There are real optimization opportunities in the VM, but they are a different
class of change and a much smaller multiplier than the tracer gap.

### 5.1 Candidate improvements

- Remove or amortize the per-instruction `heap.ShouldRunGCCycle()` check.
- Avoid the `vm.callCounts` map update on every call when JIT is disabled or the
  chunk is not JIT-eligible.
- Reduce allocations in `Call`, call-frame setup, argument handling, and
  function cloning.
- Cache decoded instruction boundaries instead of repeatedly decoding the
  remaining bytecode suffix in `jit.InstructionEnd`. The untraced profile shows
  this is material on bridge-heavy workloads; Game of Life records 413,696
  bridges.
- Add compiler superinstructions/peepholes for common sequences such as
  `GET_LOCAL`, `GET_LOCAL`, `ADD`, `SET_LOCAL`, or compare-and-jump.
- Improve allocation/GC behavior in the heap and object paths.
- Improve JIT coverage, OSR eligibility, and bridge handling. The JIT can be a
  huge win when it applies: Fibonacci `fib(40)` takes over 120 s with JIT off
  and about 3.2 s with JIT auto on this host.

### 5.2 Why this is not the first fix

The current JIT is not a uniform win on the failing examples:

| Workload | JIT stats | JIT off | JIT auto |
| --- | --- | ---: | ---: |
| Mandelbrot | `compiled=0`, no OSR entry | 0.156 s | 0.202 s |
| Game of Life | `compiled=1`, **413,696 bridges** | 0.643 s | 1.519 s |
| Eight queens | `compiled=1`, **164,426 bridges** | 0.070 s | 0.543 s |
| Fibonacci | `compiled=1`, no bridges | >120 s | 3.197 s |

Game of Life and eight queens are currently **2.4× and 7.8× faster with the JIT
disabled** because they spend their time crossing the JIT bridge. Mandelbrot
never compiles at all. Making those workloads JIT-friendly is a substantial
project with high regression risk, and it would not help Mandelbrot until OSR
can handle its loop body.

Even a hypothetical 2× reduction in total traced execution—much larger than a
2× interpreter-only improvement would deliver—would still leave roughly 12.8 s
for Mandelbrot and 14.4 s for Game of Life. Snapshot construction is largely
independent of opcode dispatch, so VM speedups do not necessarily halve traced
runtime. To pass the deadline with VM work alone, those workloads need at least a
2.5–3× speedup on this host, plus margin for the microVM. That is a much larger
and less predictable change than fixing the tracer's obvious wasted work.

### 5.3 When VM work is still worth doing

After the tracer is fixed, bytecode/JIT work is the right next performance
project: it improves every run, not only traced runs. In particular, fixing the
bridge-heavy JIT cases would improve both normal execution and trace time. But
it should not be the first move for the web deadline.

## 6. Option B: tracing implementation improvements

### 6.1 B0: Budget-aware early-out with preserved trace semantics (highest priority)

The recorder already drops step frames after `normalLimit`. Once that happens,
building more step snapshots is pure waste. However, `traceStep` also flushes
queue events, refreshes the VM state used by later event frames, and provides the
next capture point for output. The early-out must skip only the step snapshot.

Required behavior:

- **Recorder ownership:** expose a cheap "step budget exhausted" state from the
  recorder. Set it only when `AppendFrame` rejects a `step` frame; a rejected
  event or JIT frame must not disable future step capture. Recorder ownership
  also ensures sub-VMs stop when the shared budget is full.
- **Event processing:** after exhaustion, continue flushing queue events and
  emitting event/JIT frames into the reserved tail. The existing job, timer,
  await, and JIT paths already flush; avoid adding a per-instruction lock by
  checking a cheap pending-event count first.
- **Event VM state:** retain the latest frame/chunk/op position cheaply and build
  a snapshot lazily when an event frame actually needs a VM view. Simply
  returning from `traceStep` leaves `LastVM` stale.
- **Output capture:** output deltas must still reconstruct the full output after
  truncation. Event frames can carry the delta, but tests should pin down event
  attribution if that behavior is part of the animation contract.
- **Accounting:** skipping `AppendFrame` would otherwise change
  `total_frames`, `dropped_frames`, and `seq`. Preserve them with cheap
  batched/atomic accounting for skipped steps and the next sequence number, or
  version the semantics and update the frontend copy and types.
- **Limits:** keep the reserved event/JIT tail and final `end` frame behavior
  unchanged.

The timing prototype used a per-VM flag set when `AppendFrame` rejected a step
frame. It reduced Mandelbrot from 25.7 s to 0.49 s, Game of Life from 28.8 s to
1.9 s, and eight queens from 7.9 s to 1.0 s, while retaining the same stored
frame counts and nearly the same payload. It did not preserve the requirements
above: truncation counters changed, and event frames after truncation could show
stale VM state or attach output to an earlier event. Section 10 shows both
effects.

This is still a small, API-compatible optimization. The prototype is a timing
proof, not the production implementation; correctness tests for events after
truncation, output reconstruction, counters, and sub-VMs are part of B0.

### 6.2 B1: Remove locks and repeated lookups from the hot path

The tracer is currently designed as if it were called once per event; it is
actually called once per bytecode instruction. Safe hot-path changes:

- Cache the active recorder pointer and immutable limits in the VM/trace
  context. `Limits` never changes after `NewRecorder`, so `CurrentLimits` should
  not take a mutex.
- Use an atomic pointer for the active recorder instead of a global `RWMutex`.
- Avoid `TakeQueueEvents` when the pending count is zero.
- Reuse the last queue/event-loop/timer snapshot until a queue/timer/job event
  invalidates it. In a tight loop these structures usually do not change between
  instructions.
- Avoid the `SetLastVM` / `LastOutputLen` mutex round trip on every step; keep
  the last VM locally and publish it only when an event frame needs it.
- Preallocate frame and stack-view storage. Reuse scratch buffers only when the
  retained snapshots own copied data; the recorder keeps snapshot pointers and
  slices, so aliasing mutable scratch storage would overwrite older frames.

These changes do not alter the schema. The profile suggests they can remove a
meaningful fraction of the remaining per-step cost after B0, but the profile was
taken before B0. Re-profile first and drop or reorder items that no longer
matter.

### 6.3 B2: Make value snapshots cheaper

`valueView` is roughly a third of traced CPU before B0. The biggest potential
wins are:

- Fast paths for small integers and common doubles before calling the general
  `native.String` formatter.
- Fast quoting/truncation for ASCII strings without `[]rune` conversion.
- Cache display/type information where values are immutable (numbers, strings,
  null/undefined/booleans) and reuse object display data where mutation cannot
  affect the rendered display.
- Preallocate stack/frame views. Do not pool their backing storage for reuse
  unless each retained frame receives an owned copy, because the recorder keeps
  those slices.

This preserves the current frame schema and is a natural follow-up to B1, but
its payoff must be re-measured after B0 reduces snapshot volume.

### 6.4 B3: Better trace coverage and smaller payloads

B0 makes long runs finish, but it still means "record the first ~9,000 steps,
then show only events until the end". That is no worse than today, but it is not
a great animation for a multi-second run. Three directions are worth considering:

- **Sampling/decimation:** record every instruction at first, then increase the
  stride once half the budget is consumed, ending with evenly spread frames
  rather than only the beginning. This needs a small schema/UI decision around
  `seq` gaps and truncated/sampled status.
- **Delta encoding / omitted-unchanged fields:** carry forward unchanged stack,
  queue, timer, and event-loop state instead of repeating full snapshots. The
  frontend already carries the last VM view across event frames, so extending
  that idea to unchanged fields is natural. This would also cut the 39–61 MiB
  payloads dramatically.
- **Compression/streaming:** out of scope for the immediate deadline, but worth
  revisiting if payload transfer becomes the next bottleneck.

These are larger changes than B0/B1 and should follow the cheap wins, not block
them.

### 6.5 Risks

- Early-out must be recorder-scoped and thread-safe across sub-VMs.
- Do not skip `traceStep`'s event flushing, VM-state refresh, or output capture;
  skipping the whole function produces stale event frames and misplaced output
  deltas even when the retained frame count matches.
- Preserve truncation accounting or explicitly version the semantics. The timing
  prototype changed `total_frames`, `dropped_frames`, and `seq`.
- Retained snapshots must own their frame/stack-view storage before any buffer
  reuse or pooling is introduced.
- Snapshot caching must be invalidated on queue/timer/job events; otherwise the
  animation can display stale queues.
- Sampling/delta encoding changes the frontend contract and needs coordinated
  tests with `ExecutionAnimation.svelte` and the trace recorder tests.
- Regression coverage must include an event that occurs after the step budget is
  exhausted, output after truncation, low `--trace-max-frames`, and a traced
  sub-VM.

## 7. Head-to-head

| Criterion | VM/bytecode work | Tracing work |
| --- | --- | --- |
| Current gap to close | 15–140× on failing examples | 15–140× is the cost itself |
| Expected near-term gain | Typically 1.1–2×; JIT can be much larger but is workload-dependent | **Roughly 8–57× on interpreted-heavy examples in the timing prototype**; more from lock/value/payload work |
| Effort for first meaningful win | Medium–high (JIT/OSR/compiler changes) | **Low–medium** (budget check, event-state refresh, accounting, tests) |
| Risk | High: execution semantics, JIT deopts, differential tests | Medium for B0 because of event/output bookkeeping; medium for schema changes |
| Effect on all examples | Does not help Mandelbrot until OSR/JIT coverage improves | Helps every traced interpreted workload |
| Trace fidelity | Neutral (or worse if more code becomes JIT placeholders) | B0 preserves stored frames and schema only if its side effects and accounting are carried over; B1/B2 preserve schema |
| Impact outside tracing | Improves normal runs too | Only traced runs |

The decisive point is that the tracer is spending most of its time on work whose
result is discarded. VM optimization makes each op cheaper but still pays the
trace cost for every op. Tracing work removes or amortizes that cost directly.

**Verdict:** tracing work is both more beneficial and more likely to produce a
meaningful change in the web demo deadline. It targets the dominant cost, can
deliver an order-of-magnitude improvement with a small but correctness-sensitive
first change, and helps every interpreted workload. VM/bytecode optimization is
still valuable, especially for normal untraced runs and for the bridge-heavy JIT
cases, but it is a second step, not the fix for this deadline. Re-profile after
B0, because untraced execution then becomes the majority of the remaining time
on several examples.

## 8. Recommended plan

1. **P0 — budget-aware early-out with preserved trace semantics.** Land first.
   Acceptance:
   - All four web examples complete with the exact web flags in under 10 s with
     comfortable margin on the verification host and in the one-vCPU/512-MiB
     constrained proxy. Validate the same command in Firecracker before claiming
     the web deadline is fixed; host and container timings are not a substitute.
   - Stored frame count stays the same for already-truncated runs, payload stays
     within the current per-example size apart from a few KiB of timing noise,
     `seq` remains monotonic, and output deltas reconstruct the full output.
   - Queue events, job transitions, JIT frames, and the final `end` frame still
     appear after truncation, and event frames do not carry stale VM state.
   - `total_frames` and `dropped_frames` keep their current meaning, or the
     schema version, frontend copy, and tests change together.
   - Shared recorder state prevents sub-VMs from continuing step snapshots.
   - `go test ./trace ./vm` and `./test.sh` pass, including new event-after-
     truncation and output-reconstruction tests.
2. **P1 — trace hot-path cleanup.** Re-profile after P0. Then consider recorder
   pointer/limits caching, an atomic active flag, a pending-event fast path,
   cached queue/event-loop/timer snapshots, and fewer per-step mutexes and
   allocations.
3. **P2 — cheaper value views.** Re-measure first; then try fast integer/string
   formatting, preallocated owned view storage, and object display caching.
4. **P3 — trace coverage/payload.** Adaptive sampling/decimation and/or delta
   encoding, coordinated with the frontend. Payloads remain roughly 39–61 MiB.
5. **P4 — VM/JIT performance.** Start with the bridge-heavy JIT cases, caching
   decoded instruction boundaries, and Mandelbrot's OSR eligibility; add
   benchmark coverage for both normal and traced runs.

If only one change is made now, make it P0. It has by far the largest effect, but
implement it with the event, output, and accounting tests above; the existing
prototype is only a timing proof. It directly addresses the current deadline
failure without changing the trace schema.

## 9. Reproduction

Build:

```sh
cd runtime
go build -o /tmp/go_js .
```

Run an example with the web flags:

```sh
/tmp/go_js --structured --trace-execution --trace-max-frames=10000 \
  ../web/frontend/src/lib/examples/mandelbrot.js > /tmp/mandelbrot-trace.b64
```

Run without tracing for the baseline:

```sh
/tmp/go_js -structured=false -jit=auto \
  ../web/frontend/src/lib/examples/mandelbrot.js > /dev/null
```

Useful diagnostics:

```sh
/tmp/go_js -structured=false -jit=auto -jit-stats \
  ../web/frontend/src/lib/examples/game-of-life.js > /dev/null
```

The early-out timing can be reproduced by guarding `traceStep` with a flag that
is set when `trace.AppendFrame` rejects a `step` frame, then rebuilding and
rerunning the commands above. Do not merge that guard as-is: it bypasses event
flushing and VM-state refresh, fails to account for skipped steps, and can attach
output to the wrong event.

## 10. Peer review verification

The review reran the benchmarks and inspected the retained JSON with fresh
binaries built from the base commit. The prototype was the saved throwaway copy
described in Section 2; its only source differences were the early-return flag in
`vm/trace.go` and the flag field in `vm/vm.go`.

### 10.1 Fresh timings

Medians of three runs on the same host. Every run used structured output; the
baseline used `-structured=false`, and traced runs used the exact web flags with
`-jit=auto`.

| Example | No tracing | Current trace | Early-out prototype |
| --- | ---: | ---: | ---: |
| Mandelbrot | 0.16 s | 22.12 s | 0.39 s |
| Game of Life | 1.18 s | 23.60 s | 1.44 s |
| Eight queens | 0.44 s | 6.47 s | 0.84 s |
| Fibonacci | 2.67 s | 3.44 s | 3.06 s |

These reproduce the original conclusion: tracing is the dominant cost on the
interpreted-heavy examples, and early-out removes most of it.

### 10.2 Constrained proxy

The same command was run in Docker with one pinned CPU and a 512-MiB memory
limit. This is a proxy for, not a replacement for, Firecracker validation.

| Example | Current trace | Early-out prototype | Prototype peak memory |
| --- | ---: | ---: | ---: |
| Mandelbrot | 31.70 s | 0.53 s | 385 MiB |
| Game of Life | 34.97 s | 2.49 s | 420 MiB |
| Eight queens | 9.72 s | 1.27 s | 429 MiB |
| Fibonacci | 3.70 s | 3.12 s | 439 MiB |

The prototype clears the 10-second deadline in this proxy, but Fibonacci leaves
only about 73 MiB of headroom under the 512-MiB limit. Peak memory and payload
transfer need to be checked in the real microVM before treating the deadline as
solved.

### 10.3 Trace equivalence

The prototype matched stored frame counts and payload sizes closely, but not the
truncation metadata:

| Example | Current total / dropped | Prototype total / dropped | Stored frames |
| --- | ---: | ---: | ---: |
| Mandelbrot | 2,522,077 / 2,513,076 | 9,003 / 2 | 9,001 |
| Game of Life | 2,739,065 / 2,729,065 | 21,931 / 11,931 | 10,000 |
| Eight queens | 715,179 / 705,179 | 32,251 / 22,251 | 10,000 |
| Fibonacci | 45,327 / 36,264 | 9,065 / 2 | 9,063 |

Fibonacci also had one differing step VM snapshot and one differing output-delta
attachment. The larger semantic issue appeared when an event occurred after the
step budget was exhausted:

```js
let value = 0;
for (let i = 0; i < 100; i++) {
    value += 1;
}
queueMicrotask(() => console.log("microtask", value));
value = 999;
console.log("after enqueue", value);
```

With `--trace-max-frames=100`, both versions stored 94 frames and both final
outputs were correct. In the current trace, the microtask enqueue event appears
at sequence 1620 with `OP_CALL` and no output. In the prototype it appears at
sequence 90 with stale `OP_POP` state, and the later `after enqueue 999\n` delta
is attached to that enqueue event. This is exactly why B0 must preserve event
flushing, event-state refresh, output attribution, and counter semantics.

### 10.4 Other checks

- The saved Mandelbrot CPU profile reproduced the reported hotspots:
  `traceStep` 57.17%, `snapshotVM` 53.56%, `valueView` 34.01%,
  `CurrentLimits` 15.52%, GC mark work 38.48%, and `growslice` 10.17%
  (cumulative samples).
- JIT statistics reproduced `compiled=0` for Mandelbrot, 413,696 bridges for
  Game of Life, 164,426 for eight queens, and zero bridges for Fibonacci.
- The untraced profile shows repeated bytecode decoding through
  `jit.InstructionEnd` as a material bridge cost, supporting the P4 candidate.
- `go test ./trace ./vm` passed.
- `./test.sh` passed 506/506 cases, the AOC puzzle, and all benchmark checks
  before any tracing change.

### 10.5 Review decision

The tracer finding stands, and tracing should be fixed first. The implementation
decision changes from "add a one-line early-out" to "add a recorder-scoped
early-out that skips only step snapshots and preserves event state, output
deltas, counters, and sub-VM behavior." That corrected P0 remains smaller and
more predictable than VM/JIT work, but it needs the regression coverage listed
in Section 8 before it is safe to land.
