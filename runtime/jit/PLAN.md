# JIT Expansion Plan — amd64

**Status:** implemented for Linux amd64. The roadmap checkboxes below record what
shipped; §9 collects the handoff result, measurements, and deferred work.
**Scope:** `runtime/jit`, its integration points in `runtime/vm`, and `runtime/chunk`.
**Target:** Linux amd64 (SysV calling convention). arm64, Windows, and macOS
JIT-permission differences are explicitly out of scope.
**Audience:** the next implementation agent.

The interpreter remains the reference implementation. A JIT optimization is only
acceptable if it is observably identical to the interpreter, including errors,
stack overflow, coercion, and side-effect order.

---

## 0. Definition of "adequate"

This plan does **not** try to match V8. For this project, "adequate" means:

1. No semantic divergence. Every program produces the same output and error as
   the interpreter, with JIT forced on. No crashes, hangs, or corrupted VM state.
2. Numeric code is genuinely compiled: arithmetic, comparisons, locals, globals,
   branches, loops, and returns execute as machine code rather than bytecode
   dispatch.
3. Hot loops are optimized even when the containing function is called only once
   (top-level `main`, module bodies, one-shot callbacks). This requires OSR.
4. Function calls are correct. Generic calls may initially go through a safe
   runtime bridge; common calls should become direct/JIT-to-JIT calls later.
5. Anything not natively supported exits or bridges safely. Unsupported code
   must never be "compiled incorrectly".
6. Stack, GC, and executable-memory handling are safe. There is no reliance on
   undefined behavior of the Go runtime.
7. The debug/disassembly path used by the web frontend keeps working.

Non-goals for the first adequate version: full try/catch/finally in JIT,
async/generator/await JIT, proxy traps in JIT, regexp JIT, and object/string
performance parity with V8.

---

## 1. Current implementation

At the time of writing (HEAD `b7a33a0`), the JIT is a proof of concept aimed at
the recursive Fibonacci benchmark.

| Area | Location | Current behavior |
| --- | --- | --- |
| Trigger | `runtime/vm/vm.go` `Call` (~2691) | JIT only when `currentIp != nil`, not spread, call count `> 4096`, and `jit.IsJittable` |
| Eligibility | `runtime/jit/compiler.go` `checkJittability` (~779) | Linear scan; rejects `arguments`/rest; rejects any non-number constant/global; whitelists 8 opcodes |
| Compiler | `compiler.go` `compileFunction` (~353) | Emits raw x86-64 bytes directly from bytecode |
| Registers | `compiler.go` | XMM1–XMM5 are the entire virtual stack; XMM0 is the return register |
| Calls | `compiler.go` `OP_CALL` | Any call is compiled as a call to the current function; the callee value is discarded |
| Entry | `createJITFunction` + `jit_amd64.s` `jitcall` | A fake Go `func()` value whose function word points at a trampoline; JIT code runs on the Go goroutine stack |
| Code memory | `NewAssembler` | One 4 KiB RWX `mmap` per function; `mprotect` toggled to patch per-call pointers |
| Value model | `runtime/value/value.go` | 64-bit NaN-boxed values: doubles plus tagged object handles, booleans, null, undefined |
| Bytecode | `runtime/chunk/chunk.go` | 160 opcodes |
| Debug | `vm/debug.go` | Structured disassembly of compiled chunks for the web UI |

Supported opcodes: `OP_CONSTANT`, `OP_GET_LOCAL`, `OP_GET_GLOBAL`, `OP_ADD`,
`OP_SUBTRACT`, `OP_LESS_THAN_EQUAL`, `OP_CALL`, `OP_RETURN`, plus
`OP_JUMP_IF_FALSE` only when fused with `OP_LESS_THAN_EQUAL`. That is 8 of 160
opcodes, plus one fused pattern.

Everything else is either rejected up front or fails compilation. A failed
compilation is not cached, so some functions retry compilation on every call.

### 1.1 Known current miscompilations

These were reproduced against the current tree. They are not hypothetical.
Each should become a regression test. To confirm a function actually reached the
JIT, run with `-structured`: `debug_info[fn].jit` is non-empty for a compiled
function, and `--debug` prints `-- CALLING JIT FUNCTION --`.

| Case | Expected | Current behavior |
| --- | --- | --- |
| `function le(n){ if(n<=1) return 1; return 0; }` hot, then `le(NaN)` | `0` | `1`; `ja` after `ucomisd` treats unordered as "not greater" |
| Hot `f(x){ return x + g }` where `g` is later changed from a number to `"z"`, then `f(1)` | `"1z"` | a wrong value (observed `"Function Map"`); cached global assumptions are never guarded |
| Hot `f(a,b){ return a+b }`, then `f(1)` | `NaN` | `"<fn f>"`; JIT runs before missing arguments are padded |
| `apply(g,x){ return g(x) }` hot with a function argument | calls `g` | self-recursion / fatal runtime error; every `OP_CALL` becomes a self-call |
| Zero-argument jitted function whose result is consumed | result value | `undefined`; result is stored but not counted on the VM stack |
| Hot `add1(x){ return x+1 }`, then `add1("abc")` | `"abc1"` | `"abc"`; string/object bits are treated as doubles |

The same class of bug exists for every opcode: the compiler optimistically
assumes values are numbers based on the state at first call and never verifies
that assumption later.

---

## 2. Gap analysis, in priority order

### G1. No guards and no deoptimization

This is the root cause of most correctness problems. `checkJittability` inspects
the current global values and constants once, then caches that decision forever.
Globals can be reassigned; arguments and locals can be any type. A JIT needs
optimistic assumptions plus a guard per assumption and a way to reconstruct
interpreter state when a guard fails.

**Required:** a deopt path with bytecode-IP mapping and exact stack/local state,
or a runtime bridge (see G5) that performs the operation with full VM semantics.

### G2. Calls are not real calls

`OP_CALL` always emits a call to the current function entry. It works only for
direct self-recursion. It miscompiles indirect calls, calls to other functions,
methods, closures, `this`, and constructor calls. Calls also bypass the VM call
frame, recursion limit, exception handling, and argument setup.

**Required:** a call protocol. The safe first implementation is a runtime bridge
that asks the VM to perform the call. Direct/JIT-to-JIT calls are an optimization
after the bridge is correct.

### G3. Loops and locals are barely covered

The compiler has no `OP_SET_LOCAL`, `OP_DEFINE_LOCAL_SLOT`, `OP_POP_LOCAL`,
`OP_JUMP`, `OP_MULTIPLY`, `OP_DIVIDE`, or general comparisons. A normal numeric
loop such as:

```js
function sum(n) {
  let total = 0;
  for (let i = 0; i < n; i++) total = total + i * 2;
  return total;
}
```

does not JIT at all. Worse, top-level benchmark loops are in `PROGRAM_MAIN`,
which is called only once, so the 4096-call trigger can never fire. Without OSR,
the JIT cannot accelerate any of the benchmark scripts except recursive fib.

**Required:** complete numeric locals/control flow, then loop-header OSR.

### G4. The virtual stack and register model do not scale

The evaluator stack is limited to five XMM registers. There is no register
allocator, no spill slot calculation, no basic-block model, and no validation
that stack depths agree at jump targets. The `jumpstack` fixup is package-global
and shared between compilations. A function with more than five live values fails
to compile; a branch whose target layout differs can corrupt the register stack.

**Required:** a CFG, explicit stack-depth analysis, per-assembler labels/fixups,
frame/spill layout, and a register allocation strategy.

### G5. Heap values, calls, allocation, and GC are unsafe

Current JIT code cannot allocate, call helpers, touch objects, or throw. That is
why it is limited to numbers. Adding a Go call from JIT code while a JIT frame is
live is not safe without additional runtime support:

- Go stack growth copies the goroutine stack and invalidates JIT `RSP`/`RBP` and
  saved JIT return addresses.
- GC stack scanning needs function metadata for every PC. Dynamically generated
  code is not in Go's `pclntab`, so a GC that walks a stack containing JIT frames
  cannot reliably unwind it.
- Async preemption rejects unknown PCs, but the JIT must not depend on that
  accident for correctness.
- Go's internal ABI is not a stable interface for hand-written call sites.

**Required:** define and enforce a hard invariant: **JIT machine code never calls
Go code while a JIT frame is live.** All heap/allocation/coercion/call/throw work
goes through a return-to-Go bridge, and the bridge re-enters JIT at a continuation
address. This uses the ordinary goroutine stack and needs no runtime changes.

### G6. Stack overflow and recursion limits

JIT self-recursion uses native `call` without a guard. It bypasses the VM's
`FRAMES_MAX` counter and has no native stack limit, so deep recursion through a
hot function is a latent crash rather than the interpreter's `RangeError`. Long
straight-line JIT code has no safepoints, so a GC requested by another goroutine
can wait indefinitely.

**Required:** a depth/stack guard and a loop budget that exits to Go periodically.

### G7. Code cache, ownership, and lifetime

`jittedFns`, `jittableFns`, `jumpedChunks`, `jumpstack`, and `debugs` are package
globals. They are not synchronized, are keyed by `object.Callable` (so function
clones compile separately), retain functions forever, and have no size limit.
Each function consumes a whole page. Re-entry patches the code in place.

**Required:** a per-VM `JITContext`, keyed by `*value.ValueChunk`, with a bounded
code arena, cached failures, and no strong references that prevent function
collection. Dynamic per-call data belongs in an activation, not in patched code.

### G8. Code generation infrastructure

The current emitter writes raw bytes with hand-written prefixes and no verifier.
There are no labels, no general fixup list, no instruction encoder, no frame-size
calculation, no bounds checks on the 4 KiB buffer, and no alignment discipline.
`PAGE_SIZE` is hardcoded to 4096.

**Required:** a small typed encoder, label/fixup mechanism, frame builder, code
arena, and compile-time validation.

### G9. Test and observability gaps

There are no JIT-specific tests in `test/test-scripts/` today. The test runner
does not compare JIT-on and JIT-off modes. The only JIT artifact is disassembly.
Known miscompiles above are not covered.

**Required:** force-JIT mode, differential test runs, per-opcode unit tests,
stack/GC stress tests, and performance tracking.

---

## 3. Target architecture

### 3.1 Core invariant

> JIT code never calls Go code while a JIT frame is live.

A JIT execution is a **segment**. A segment runs machine code until one of:

- it completes and returns a value;
- it hits a guard/unsupported operation and requests a **deopt**;
- it needs a runtime service and requests a **bridge operation**;
- it exhausts its loop budget and requests a **safepoint**;
- it hits the stack/depth guard and requests a **throw**.

The bridge returns to Go, which performs the operation, then either re-enters the
same compiled function at a continuation address or resumes the interpreter.

This is deliberately less ambitious than a native JIT that calls the Go runtime
directly. It is safe with the existing Go runtime and can be built incrementally.
Direct JIT-to-JIT calls are an optimization on top, not the foundation.

### 3.2 Proposed execution flow

```text
vm.Call / vm.run
      |
      v
  runJIT(activation)  <---------------------------+
      |                                           |
      v                                           |
  jitEnter(entryPC, activation)                   |
      |                                           |
      v                                           |
  machine-code segment                            |
      |                                           |
      +-- DONE ---------> copy result to vm.stack  |
      +-- DEOPT --------> reconstruct vm frame ---> interpreter resumes
      +-- BRIDGE -------> Go helper (call/property |
      |                    /coercion/allocation)   |
      +------------------> re-enter at continuation
```

### 3.3 Activation

A `JITActivation` is a Go-heap object, reachable from `runJIT`'s Go stack frame
for the lifetime of a segment. It contains:

| Field | Purpose |
| --- | --- |
| `chunk *value.ValueChunk` | Immutable code identity |
| `fn object.Callable` | Heap scope, `this`, callable identity |
| `globals []value.Value` / base pointer | Refreshed at every entry; the slice may reallocate |
| `constants []value.Value` / base pointer | Per-chunk constants |
| `localStart int` | Location of this frame in the VM stack |
| `values []value.Value` | Locals and evaluator stack for the segment |
| `localCount`, `stackDepth` | Frame layout |
| `bytecodeIP` | Current interpreter IP for deopt/bridge |
| `request` | Bridge op id and arguments |
| `result` / `thrown` | Bridge result |
| `entryPC` / `continuationID` | Re-entry point |
| `depth`, `stackLimit` | Recursion/stack guard |
| `loopBudget` | Remaining iterations before a safepoint |

The exact field offsets are part of the JIT ABI. Freeze them with `unsafe.Offsetof`
unit tests; do not let the Go compiler change the layout silently.

All live `value.Value`s that must survive a bridge call live in the activation
(or in `vm.stack`) before the bridge returns to Go. The activation is a normal Go
object, so it stays alive across the bridge, but its `value.Value` elements are
just 64-bit handles into this project's own mark-and-sweep heap; the Go GC does
not trace them. Before any bridge call that can allocate or run a GC, make the
activation's `values` visible to that collector: copy them to `vm.stack`, or add
a scoped root API around the existing `heap.PushToRoots` (there is currently no
pop operation). Do not rely on Go GC scanning alone for handle liveness. After a
bridge re-entry, refresh the activation's globals and any slice base pointers; a
helper may have grown `globals` or `vm.stack`.

### 3.4 ABI and entry trampoline

- JIT-internal code follows SysV AMD64: caller-saved `RAX, RCX, RDX, RSI, RDI,
  R8-R11`, `XMM0-XMM15`; callee-saved `RBX, RBP, R12-R15`.
- Reserve `RBX` for the activation pointer, `R12` for globals, `R13` for
  constants, `R14` for the stack limit, and `R15` as a scratch/deopt register.
  Treat this as an ABI; document any change.
- `jitEnter(entryPC uintptr, activation *JITActivation) JITStatus` is a small
  Go assembly function with `NOSPLIT`. It saves Go callee-saved registers,
  aligns the stack for SysV, places the activation in `RDI`, calls the JIT entry,
  restores registers, and returns the status in `RAX`. Values, bridge arguments,
  and errors travel through the activation, not through ambiguous XMM/RAX
  conventions.
- Replace the fake `func()` closure scheme (`createJITFunction`/`jitcall`) with
  this explicit entry point. It is easier to audit and no longer depends on Go's
  closure ABI.
- A JIT function entry performs a stack-limit check before allocating its frame:
  `if RSP - frameSize < stackLimit -> stack-overflow exit`.
  `stackLimit` comes from either the goroutine's real stack bounds (read in
  assembly) or a fixed reserved budget established by `runJIT`; document which.
  Also enforce the JS frame limit with an activation depth counter.
- On stack-overflow/abort, restore the saved entry `RSP`/`RBP` and return a
  `THROW` status with a `RangeError`; do not try to unwind each JIT frame
  individually. The Go caller handles the throw exactly like an interpreter
  throw.

### 3.5 Compiler pipeline

```
bytecode -> decoder -> CFG + stack-depth verifier -> IR -> register alloc -> encoder -> code arena
```

1. **Decoder.** A complete operand-width table for all 160 opcodes. It must agree
   with `ReadOp` and the VM dispatch. Unit-test every opcode's width.
2. **CFG.** Leaders are entries, jump targets, and instructions after jumps.
   Compute evaluator-stack depth at every block entry. Reject or deopt on
   disagreement, underflow, or overflow. Unsupported instructions terminate a
   block with a deopt exit; they do not fail the whole function.
3. **IR.** Stack operations are lowered to virtual registers or activation slots.
   Keep simple peepholes: constant folding, redundant guard removal, compare +
   jump fusion, branch-to-next elimination.
4. **Register allocation.** Locals live in activation slots; keep the top N
   evaluator values in registers within a block, spill at block boundaries,
   calls, and exits. Use XMM registers for number math and GPRs for tagged values.
   The spill/stack layout must be deterministic from the CFG.
5. **Encoder.** Typed instruction helpers (`MovsdLoad`, `Addsd`, `Ucomisd`,
   `Jmp`, `Jcc`, `Call`, `Ret`, ...), labels, and a fixup list owned by the
   assembler. No package-global jump stack. All branch displacements are 32-bit
   near jumps; do not implement relaxation initially.
6. **Code arena.** Allocate executable pages from a per-context arena; write with
   RW, then `mprotect` to RX. Never map RWX. Write only freshly allocated pages
   that cannot be executing yet; do not toggle permission on a page that may be
   running. Check every write against the page bound. Use `os.Getpagesize()`, not
   a 4096 constant. Bounded total code size with a clean compile failure when
   full.

### 3.6 Types, guards, and deopt

Values are NaN-boxed `value.Value`s. The compiler should not assume a type
without a guard unless the value is a compile-time constant or a proven local
type.

- **Number guard:** must exactly match `value.Value.IsNumber`, including NaN.
  Canonicalize arithmetic NaN results to `STANDARD_NAN` when materializing a
  value, or use a tag test that cannot confuse a quiet NaN payload with a tag.
- **Boolean/null/undefined:** compare against the exact tagged values.
- **Objects/strings:** treated as handles; their type is checked only when a
  fast path needs it.
- **Every dynamic operand** is guarded at use. Static type inference is an
  optimization, not a correctness requirement.
- On a failed guard, use one of two exits:
  - **deopt** when resuming the interpreter at the current bytecode IP is the
    simplest correct behavior;
  - **bridge** when a runtime operation can produce a result and re-enter
    (e.g. string `+`, property get, generic call).

A deopt entry records: JIT PC, bytecode IP, stack depth, local layout, and any
register-to-slot mapping. On deopt, `runJIT`:

1. copies activation locals and evaluator values into `vm.stack` at `localStart`;
2. sets `vm.stackTop = localStart + localCount + evalDepth`;
3. sets the current frame's `startIp = bytecodeIP`;
4. returns `(f, c)` to `vm.Call`, which sets `*currentIp = bytecodeIP` and leaves
   the frame pushed so `vm.run` continues interpreting that frame.

Zero-argument calls need their own fix: after a successful JIT return set
`vm.stackTop = localStart + 1` and store the result at `vm.stack[localStart]`.
`popN(argCount - 1)` is wrong when `argCount == 0`.

### 3.7 Runtime bridge

The bridge is the safe equivalent of "call a helper". A request contains an op id
and a small fixed argument area in the activation. The Go side dispatches to the
same VM methods the interpreter uses, so semantics stay in one place.

Initial request set:

| Category | Requests |
| --- | --- |
| Arithmetic/coercion | non-number `ADD` (string/BigInt/mixed), `MODULO`, `EXPONENTIATION`, `TO_NUMBER`, `TO_STRING`, `TO_PROPERTY_KEY`, bitwise conversions |
| Properties | `GET_MEMBER`, `SET_MEMBER`, `DELETE`, `HAS_PROPERTY`, array index/length helpers |
| Calls | `CALL`, method/this calls, `NEW`, optional calls, spread calls |
| Allocation | object/array creation, `PUSH_ELEMENT`, string builder append |
| Exceptions | throw a value, stack overflow, explicit `THROW` |
| Safepoints | check GC/loop budget, run a GC cycle if requested |

For calls, `vm.callValue` is the safest first implementation: it already creates
a sub-VM and runs the callee to completion. The caller JIT frame is not live
while that happens because the bridge returned to Go. Note that `callValue` is
also used by native callbacks and does not exactly mirror every `OP_CALL` detail
(sloppy `this`, script-mode globals, method handles); add a `callFromJIT` wrapper
that mirrors `OP_CALL` where needed. Later, a shared JIT context can let the
callee itself be JIT-compiled.

Bridge/exception integration with `vm.Call` must be designed explicitly. The
current `Call` signature has no error return and relies on `throwValue` inside
the interpreter. A likely change is `Call(...) (f CallFrame, c value.ValueChunk,
err error)` with the JIT bridge using `vm.throwValue` so JS `try/catch` in outer
interpreter frames still works. This call-site refactor should be done once, in
the bridge phase, not piecemeal.

### 3.8 Calls

Phase order:

1. Bridge all calls through `vm.callValue`. Correct, safe, slower.
2. Direct self-recursion for **pure numeric** functions only. "Pure" means: no
   globals, no heap values, no runtime requests, no guards after entry, and no
   call other than itself. Fib qualifies. This keeps the existing fib win while
   avoiding nested-frame deopt complexity.
3. Direct calls to other jitted chunks with a frame stack in the activation.
   A call request becomes a machine `CALL` to the callee entry with arguments in
   registers; the callee has the same segment discipline. Deopt of a nested
   frame requires reconstructing all active frames, so do not enable this until
   the activation frame stack and deopt state for multiple frames are tested.
4. Inline caches for monomorphic call targets; megamorphic calls go to bridge.

Argument handling must happen **before** entering JIT: missing arguments are
padded with `undefined` (or `UNINITIALIZED` for default parameters), and
`arguments`/rest setup is completed. Functions using `arguments`, rest, or
default parameters remain non-jittable until the call protocol handles them.

### 3.9 OSR and tiering

Two entry paths:

- **Function entry:** invocation counter per `*ValueChunk`. Compile once, cache
  success and failure. Counter is shared across callables/clones and sub-VMs.
- **Loop entry (OSR):** backedge counter per loop header. When hot, compile the
  reachable region with an entry at the loop header and copy the live locals and
  evaluator stack from `vm.stack` into the activation.

Recommendation: compile the whole function (or all reachable blocks from the
loop header), not a separate loop function. Keep one code object per chunk; OSR
is just an alternate entry point. On normal loop exit, compiled code can continue
to the function return if supported; otherwise it deopts at the exit block.

OSR details:

- Only loops whose reachable body has no unsupported deopt site are OSR-able.
  A loop containing try/catch, iterator setup, async, etc. stays interpreted.
- `break`, `continue`, and `return` are normal control flow inside the compiled
  region. `throw` deopts before executing.
- On deopt, resume at the exact bytecode IP; never replay loop iterations.
- Backedge budget: decrement a counter on each backward branch. At zero, request
  a safepoint, run GC if needed, and re-enter. This prevents a JIT loop from
  blocking GC or event-loop progress indefinitely.
- Deopt-storm policy: if a chunk deopts more than N times in a window, stop
  trying to JIT it for the rest of the run.

Hotness must not be tied to `currentIp != nil` or to a single VM instance.
Callbacks executed through `callValue` currently get a fresh VM and fresh
`callCounts`, so a callback called thousands of times never becomes hot. Move the
counters into the shared JIT context.

### 3.10 Heap fast paths

The bridge makes objects/arrays/strings correct but not necessarily fast. Add
fast paths in this order, measuring after each one:

1. **Arrays:** direct `ObjArr.items` access for numeric, in-range, non-hole
   element reads; in-range stores; `length`. Guard object type, index integrality,
   range, and hole. Misses deopt/bridge.
2. **Objects:** this is the expensive one. The current `Members` Go map cannot be
   read directly from JIT code. Either:
   - keep property access on the bridge for now; or
   - introduce hidden shapes / dense property slots in `native.ObjObject` and
     cache `(shape, slot)` in an inline cache. This is a broad native-object
     refactor and needs its own design review.
3. **Calls:** monomorphic callsite caches.
4. **Strings:** string concatenation allocates; keep it on the bridge until the
   bridge cost is measured. A builder fast path is a later optimization.
5. **Iterators:** bridge first. If the array benchmark matters, consider a
   compiler-level lowering of `for...of` over arrays to an indexed loop, which
   also helps the interpreter and is independent of JIT.

Inline caches are data structures in the Go heap, not self-modifying code:
`{epoch, guard, slot, target}`. JIT code loads the cache address and tests the
guard; a miss takes the bridge. Add a heap epoch/generation counter and
invalidate caches when it changes, so reused handles cannot produce stale hits.

### 3.11 Code ownership and concurrency

- Create a `JITContext` per root VM (or one shared context with a mutex) and pass
  it to sub-VMs. Move all maps and code arenas into it.
- Cache compiled code by `*value.ValueChunk`. Function clones share code; dynamic
  state (`fn`, heap scope, `this`) goes in the activation.
- Use an RWMutex around compilation/cache lookups. Execution of immutable code
  needs no lock; activation state is per invocation.
- Bound the code cache (bytes and entry count). On pressure, stop compiling new
  chunks rather than evicting code that is currently executing. Do not let the
  cache keep `object.Callable` values alive.
- Cache compile failures per chunk so a failed function is not retried on every
  call.

### 3.12 Debug and observability

- Preserve `GetJittedDisassemblyForChunk` and the `JitDetail` JSON shape used by
  `vm/debug.go` and the web frontend.
- Keep Zydis strictly in the debug/disassembly path; a disassembly failure must
  never fail compilation or execution.
- Add `--jit-stats`: compiled chunks, code bytes, deopts by reason, bridge
  requests by kind, OSR entries, and compile failures.
- Add a trace mode that logs `chunk/bytecodeIP <-> machine PC` and deopt reasons.
- Do not instrument the hot path unless the flag is on.

---

## 4. Roadmap

The phases are ordered so that each one reduces risk and is independently
shippable/rollback-able. Do not start an optimization phase before the previous
phase's correctness gates pass.

### Phase 0 — Baseline, flags, and regression harness (size S)

Goal: make JIT behavior testable before changing the compiler.

- [x] Add runtime options: `--jit=off|auto|force`, `--jit-call-threshold`,
      `--jit-loop-threshold`, `--jit-max-code-bytes`, `--jit-stats`.
- [x] Expose the existing `flags.EnableJIT` through those options
      (`ECMASCRIPT_ROADMAP.md` M0 item 5).
- [x] Add the known-miscompile cases from §1.1 as tests under
      `test/test-scripts/jit/` and/or Go tests.
- [x] Add a differential runner mode: execute the same case with JIT off and
      JIT forced, compare stdout/diagnostics, and fail on divergence.
      (`GO_JS_DIFFERENTIAL=1 ./test.sh`; `jit-*` scripts always run forced.)
- [x] Record baseline timings for fib, a numeric loop, array iteration, object
      access, and string concatenation. See `runtime/jit/BASELINE.md`.
- [x] Decide the default: until Phase 2 is complete, keep JIT out of the default
      CI path or set `auto` with the existing conservative eligibility.
      Decision: default remains `auto` with the existing conservative
      eligibility; the differential suite is opt-in until Phase 2 is green.

Exit criteria: `./test.sh` still passes; the differential harness runs; the
current miscompiles are documented as failing (or JIT is disabled so they cannot
reach users).

### Phase 1 — Activation, bridge skeleton, and new codegen foundation (size L)

Goal: replace the unsafe core without changing the supported opcode set.

- [x] Introduce `JITContext`, `JITActivation`, compiled-chunk cache, and the
      `runJIT` loop.
- [x] Add `jitEnter` assembly and remove the fake `func()` entry path.
- [x] Implement the bridge protocol: request IDs, fixed argument area, statuses
      `DONE`, `DEOPT`, `BRIDGE`, `THROW`.
- [x] Implement the deopt state copy to `vm.stack` and the `vm.Call` integration
      (including a proper error path/`throwValue`).
- [x] Port the current 8 opcodes to the new encoder; unsupported ops become
      deopt sites instead of compile errors.
- [x] Add the guards needed to make the ported subset safe: number/tag checks,
      globals reload/guard, argument padding before entry, zero-argument result
      fix, and correct unordered comparison handling. If an opcode cannot be made
      safe yet, mark it deopt-only rather than porting the old assumption.
- [x] Add labels/fixups, frame layout, stack-depth analysis, RW→RX code arena,
      page-size handling, and bounds checks.
- [x] Key code by chunk; move globals into the context; cache failures.
- [x] Preserve structured disassembly.

Exit criteria: fib still works with JIT force; the known miscompiles are fixed;
no package-global maps; encoder and activation-layout unit tests pass.

### Phase 2 — Complete numeric core (size L)

Goal: compile ordinary numeric JavaScript, not just fib.

- [x] Locals: `DEFINE_LOCAL_SLOT`, `GET_LOCAL`, `SET_LOCAL`, `POP_LOCAL`,
      `GET_LOCAL_RAW`, plus stack ops `POP`, `DUP`, `DUP2`, `POP_BELOW2`,
      `PUSH_CURRENT`.
- [x] Constants/immediates: number, boolean, null, undefined; `PUSH_UNDEFINED`,
      `NILL`, `UNDEFINED`, `PUSH_UNINITIALIZED` (deopt/throw when read).
- [x] Arithmetic: `ADD`, `SUBTRACT`, `MULTIPLY`, `DIVIDE`, `MODULO`,
      `EXPONENTIATION`, `NEGATE`; numeric guards; bridge non-number `ADD`.
- [x] Comparisons: `<`, `<=`, `>`, `>=`, `STRICT_EQUALS`, `STRICT_NOT_EQUALS`,
      `EQUALS`, `NOT_EQUALS` for number/bool/null/undefined; NaN/unordered
      handling; bridge object/string comparisons.
- [x] Bitwise: `B_AND`, `B_OR`, `B_XOR`, `BITWISE_NOT`, shifts; use a bridge for
      ToInt32/ToUint32 until an exact inline sequence is tested.
- [x] Truthiness/jumps: `JUMP`, `JUMP_IF_FALSE`, `JUMP_IF_TRUE`, `NOT`,
      `LOGICAL_AND`, `LOGICAL_OR`, nullish variants where cheap; objects are
      always truthy, strings need a length check or bridge.
- [x] `UPDATE`, `TO_NUMBER`, `VOID`, simple `TYPEOF`.
- [x] Global reads with guards; global writes via bridge. Do not cache global
      values or types across calls.
- [x] Correct NaN canonicalization/comparison; add the NaN regression.
- [x] Correct zero-argument result handling; add the arity regression.
- [x] Self-recursion only for functions proven pure numeric; all other `OP_CALL`
      goes through the bridge.

Exit criteria: the `sum` example compiles and runs correctly; all numeric
differential tests and the full test suite pass in force mode; no known
miscompiles remain.

### Phase 3 — OSR and loop compilation (size M/L)

Goal: accelerate hot loops in `main` and one-shot callbacks.

- [x] Add backedge counters (per chunk/IP) and a configurable loop threshold.
- [x] Build an OSR entry that copies live state from `vm.stack` into the
      activation and starts at the loop header.
- [x] Compile reachable blocks from the loop header; handle loop exits,
      `break`/`continue`, and `return`.
- [x] Implement continuous backedge safepoints/budget.
- [x] Add deopt-storm detection and per-chunk blacklisting.
- [x] Add tests for nested loops, loop-local variables, loop exit values, empty
      loop bodies, and loops with a non-OSR-able branch.

Exit criteria: a top-level numeric loop runs in JIT; fib and the numeric loop
benchmark beat the interpreter; OSR does not replay side effects; JIT off/force
outputs match.

### Phase 4 — Real calls (size M/L)

Goal: stop treating every non-self call as a bridge.

- [x] Move argument padding/default initialization before the JIT attempt; fix
      missing-argument behavior.
- [x] Support `this`, method handles, and arrow-function lexical `this` where
      possible.
- [x] Direct self-recursion for pure numeric functions (retain fib performance).
- [ ] Direct monomorphic calls to jitted chunks with an activation frame stack;
      define and test multi-frame deopt or restrict to no-deopt callees.
      Decision: deferred. The bridge replays the real `OP_CALL` on the same VM,
      so the callee can itself be JIT compiled and all call semantics stay in
      the interpreter. No nested-frame deopt is ever required.
- [ ] Call-site inline caches; megamorphic fallback to bridge. Decision:
      deferred with the monomorphic-call work above.
- [x] `arguments`, rest, default parameters, constructors, and spread: bridge or
      mark non-jittable with a clear reason.
- [x] Share hotness/counters across sub-VMs so callbacks can become hot.

Exit criteria: mutual recursion, function-valued arguments, closures/methods,
and missing/extra arguments are correct; fib remains fast; no nested-frame
corruption.

### Phase 5 — Bridge coverage for the rest of the language (size L)

Goal: safely JIT functions containing heap/string/iterator operations without
compiling those operations natively.

- [x] Property reads/writes, array index/length, object creation, array creation,
      `PUSH_ELEMENT`.
- [x] String concatenation, `TO_STRING`, template literals, string length/index.
- [x] Iterators and `for...of`/`for...in` setup and next/current operations.
- [x] `NEW`, optional calls, spread, `IN`, `INSTANCEOF`, `TYPEOF` for non-numbers.
- [x] Keep try/catch/finally, async/generator/await, class construction,
      modules/import, proxy, and regexp non-jittable for now. Class creation
      and constructors are bridged instruction-by-instruction on the real VM
      rather than compiled natively; try/catch, async/generator, `with`, and
      completion bookkeeping reject the whole chunk.
- [x] Measure bridge overhead; only compile loops whose bodies benefit.

Exit criteria: a hot loop containing property access and string concatenation
produces correct output; performance is measured (it may still be slower than
the interpreter until Phase 6); no function with unsupported semantics is
accepted.

### Phase 6 — Fast paths and inline caches (size XL, stretch)

Goal: remove bridge overhead from the common heap operations.

- [ ] Array numeric get/set/length fast path with type/index/range/hole guards.
      Partial: the bridge now implements member get/set, `PUSH_ELEMENT`, array
      creation, and object creation directly in Go instead of replaying the
      instruction in a sub-frame. Direct `ObjArr.items` access from machine
      code is deferred while the heap still stores objects behind a Go map;
      see the design note in §7.
- [x] Object property design spike: hidden shapes + dense slots, or another
      direct-access layout. Do not start without a written design and a
      compatibility plan for descriptors/accessors/prototypes.
      Decision: keep properties on the bridge for v1. `ObjObject.Members` is a
      Go map, so machine code cannot read it without a handle-to-object table
      and a shape/slot ABI. A compatibility plan must cover accessors,
      prototypes, descriptors, `__proto__`, and GC epochs before that change.
- [ ] Monomorphic property/getter caches with heap epoch invalidation. Deferred
      with the object layout work.
- [ ] Monomorphic call-target caches. Deferred; calls go through the correct
      bridge and can reach JIT-compiled callees.
- [ ] Optional: compiler lowering of `for...of` over arrays to indexed loops.
      Deferred (would benefit the interpreter independently).
- [ ] Optional: string builder fast path for repeated `+=` in a loop. Deferred;
      the bridge preserves builder semantics.

Exit criteria: array iteration is at least as fast as the interpreter; object
property benchmarks improve or are explicitly left on the bridge; all caches
invalidate correctly across GC and mutation.

### Phase 7 — Hardening, docs, and handoff (size M)

- [x] GC stress: `GOGC=1`, forced GC from another goroutine, long JIT loops, and
      allocation-heavy bridge calls.
- [x] Stack stress: deep recursion, mutual recursion, stack guard, negative
      recursion test expecting `RangeError`.
- [x] Code cache stress: thousands of distinct chunks, code-size limit, compile
      failure fallback, no leaks of `Callable`s.
- [x] W^X verification and page-permission tests.
- [x] Race/concurrency tests for shared JIT contexts.
- [x] Fuzz the decoder/CFG and differential-test generated numeric programs.
- [x] Update `README.md` and `ECMASCRIPT_ROADMAP.md` with the supported subset,
      flags, and benchmark results.
- [x] Delete or build-tag the unused arm64 JIT stubs; keep amd64 clean.
- [x] Final pass: `./test.sh`, JIT-off vs JIT-force, benchmarks, and a short
      handoff note in this file. `GO_JS_DIFFERENTIAL=1 ./test.sh` passed
      506/506 cases in the Docker image (Go 1.26 / Node 24), including AOC and
      all benchmarks.

---

## 5. Opcode coverage matrix

"Bridge" means the operation is performed by the Go runtime and execution
re-enters the JIT. "Deopt" means the interpreter resumes at that bytecode IP.
"Never v1" means functions containing it are not compiled (or the block is not
OSR-able).

| Opcode group | Examples | Target |
| --- | --- | --- |
| Constants/immediates | `CONSTANT`, `NILL`, `UNDEFINED`, `PUSH_UNDEFINED` | Phase 2 native |
| Stack | `POP`, `DUP`, `DUP2`, `POP_BELOW2`, `PUSH_CURRENT` | Phase 2 native |
| Locals | `DEFINE_LOCAL_SLOT`, `GET_LOCAL`, `SET_LOCAL`, `POP_LOCAL`, `GET_LOCAL_RAW` | Phase 2 native |
| Globals | `GET_GLOBAL`, `SET_GLOBAL`, `DEFINE_GLOBAL` | Read native + guard; write bridge/deopt |
| Arithmetic | `ADD`, `SUBTRACT`, `MULTIPLY`, `DIVIDE`, `MODULO`, `EXPONENTIATION`, `NEGATE` | `+ - * /` native; `% **` bridge then optimize |
| Bitwise | `B_AND`, `B_OR`, `B_XOR`, `BITWISE_NOT`, shifts | Bridge first, native after exact ToInt32 tests |
| Comparison | `<`, `<=`, `>`, `>=`, `STRICT_EQUALS`, `EQUALS`, `NOT_EQUALS`, `IN`, `INSTANCEOF` | Numeric/identity native; rest bridge |
| Control flow | `JUMP`, `JUMP_IF_FALSE`, `JUMP_IF_TRUE`, `LOGICAL_AND/OR`, nullish jumps | Phase 2 native |
| Update/coercion | `UPDATE`, `MEMBER_UPDATE`, `TO_NUMBER`, `TO_STRING`, `TO_PROPERTY_KEY`, `TYPEOF`, `VOID`, `NOT` | Numeric native; rest bridge |
| Calls | `CALL`, `CALL_OPTIONAL`, `CALL_CALLEE_FIRST`, `NEW`, `SUPER_CALL`, `ARG_START`, `SPREAD` | Bridge; direct calls Phase 4 |
| Objects/arrays | `CREATE_OBJECT`, `SET/GET_OBJECT_MEMBER`, `CREATE_ARRAY`, `PUSH_ELEMENT`, `SET_OBJECT_PROTOTYPE` | Bridge; array fast path Phase 6 |
| Iterators | `GET_ITERATOR`, `ITERATOR_NEXT/CURRENT*`, close ops | Bridge; optional compiler lowering |
| Strings/classes/modules | string builders, `CREATE_CLASS*`, `IMPORT*`, `EXPORT*`, `NEW_REGEXP` | Bridge/deopt; no native code v1 |
| Exceptions | `TRY_*`, `THROW`, `CATCH_*` | Never v1 (function not compiled) |
| Async/generators | `AWAIT`, `YIELD*`, async iterator ops | Never v1 |
| Proxy/private/with | proxy traps, `GET/SET_PRIVATE_MEMBER`, `WITH_*` | Never v1 |
| Debug/misc | `STACK_MARK`, `STACK_TRIM`, `STACK_UNMARK`, completion ops | Bridge/deopt until understood |

The decoder must still know every opcode's operand width even for "Never v1"
groups, so it can build correct blocks and deopt IPs.

---

## 6. Testing plan

### Regression tests (must exist before Phase 2 is considered done)

- NaN comparison (`<=`, `<`, `>=`, `>`), `-0`, `Infinity`, integer overflow.
- Globals reassigned to different types after a function becomes hot.
- Missing, extra, and default arguments.
- Zero-argument functions whose result is consumed.
- Indirect calls, function arguments, methods, closures, constructors.
- String concatenation and mixed-type `+`.
- Deep recursion and stack overflow (`RangeError`).
- `break`/`continue`/nested loops under OSR.
- Deopt at a side-effecting operation must not duplicate the side effect.

### Differential testing

Add a runner mode that executes each test with JIT off and JIT forced
(threshold 1) and compares stdout, error class, and phase. Start with the JIT
directory and numeric tests; expand to the full suite as coverage grows. This is
the primary safety net: the interpreter is the oracle.

### Unit tests

- Decoder operand widths for all 160 opcodes vs VM `ReadOp`.
- CFG stack-depth verification: underflow, mismatched merge, unreachable blocks.
- Encoder golden bytes for representative instructions, REX prefixes, branch
  fixups, and frame layout.
- Activation field offsets and `unsafe.Offsetof` assertions.
- Deopt state reconstruction for each supported opcode.
- Code arena: page growth, bounds, W^X transition, cache eviction.
- Bridge request/result/error handling.

### Stress and fuzz

- `GOGC=1`, forced `runtime.GC`, and an allocating goroutine while a long JIT
  loop runs.
- Random numeric/control-flow programs generated from a restricted grammar;
  compare interpreter vs JIT.
- Deep recursion and mutual recursion through both native and bridge calls.
- Thousands of distinct jitted chunks; code cache limit; no retained callables.

### Performance tracking

Record, with JIT off/auto/force:

- `fib(35/40)` and the existing `naive-recursive-fib.js`.
- A top-level numeric loop (`sum`, countdown, nested loop).
- The existing array/object/string benchmarks (for correctness now, performance
  after Phase 6).

Do not make wall-clock timing a hard CI gate initially; record numbers and add
generous regression budgets once the implementation is stable.

---

## 7. Risks and open questions

| Risk | Mitigation |
| --- | --- |
| Go runtime cannot unwind JIT frames | Hard invariant: no Go calls while JIT frames live; bridge returns to Go; keep segments bounded |
| Go stack growth moves JIT frames | Same invariant; no Go calls from JIT; stack guard for self-recursion |
| GC misses object handles in JIT state | Live values are in activation/`vm.stack` before every bridge/exit; activation is Go-scanned |
| Deopt reconstructs wrong stack | Explicit stack-depth analysis, per-site metadata, differential tests |
| Object layout coupling for fast paths | Separate design review; guard types; use accessor offsets with compile-time assertions |
| Code cache grows without bound | Bounded per-context arena; cache failures; no strong refs to callables |
| Inline caches become stale after GC/handle reuse | Heap epoch/generation checks; invalidate on GC |
| Compiler bugs crash the process | Encoder verifier, bounds checks, deopt fallback, run with JIT off in CI until green |
| pprof/signal stack sampling sees unknown JIT PCs | Document that CPU profiling requires JIT off until JIT frame metadata exists; keep sampling out of production paths |
| Zydis `go:linkname` breaks across Go versions | Debug-only path, recoverable failure, no execution dependency |
| Multiple VMs share JIT state | Per-VM context or a mutex-protected shared context; no package globals |
| JIT makes `--structured` output nondeterministic | Preserve chunk-keyed disassembly; test the JSON shape |

Open questions for the next agent:

1. Should JIT remain enabled by default while Phases 1–2 are in progress, or
   should it be `--jit=auto`/off in the default test path until the differential
   suite is green?
2. Is the bridge-exit model acceptable for the first release, or is a native
   helper-call ABI with JIT stack maps worth the extra work now? This plan
   recommends the bridge.
3. Is changing `vm.Call` to return an error acceptable, or should the bridge
   use a VM-level pending-error field? The error return is cleaner but touches
   many call sites.
4. Do we want to invest in hidden shapes/dense object slots (Phase 6), or accept
   bridge-speed object access indefinitely?
5. Should `for...of` over arrays be lowered by the compiler instead of teaching
   JIT about iterators?
6. What is the maximum acceptable code cache size and JIT compile latency for the
   web demo's per-request process?

---

### 6.1 Direct heap access design note

The Phase 6 array/object fast paths were deliberately not implemented on top of
the current heap layout. Direct machine-code access to `native.ObjArr.items`
needs three things that do not exist yet:

1. A handle-to-object-pointer table. `heap.handleLayer` is a Go map, and its
   pointer table is not exposed at a stable address. Add a parallel
   `[]uint32` (`handle -> object index`) maintained by allocate, sweep, and
   defrag, and pass its base plus `handleCount` in the activation.
2. A verified object-layout ABI. The JIT needs the `object.Object` interface
   word, the `*native.ObjArr` type word, and `unsafe.Offsetof` for `items` and
   the slice header. Each must be asserted at compile time and tested.
3. Invalidation. Any bridge that can allocate, mutate, or run GC must clear the
   per-activation array cache before the next segment. The current bridge does
   this implicitly by re-entering through `jitEnter`, but that has to become an
   explicit epoch check if caches become persistent.

The safe intermediate step that *was* implemented is a bridge fast path:
`GET_OBJECT_MEMBER`, member stores, `PUSH_ELEMENT`, array creation, and object
creation perform their work directly in Go on the real VM instead of replaying
the bytecode in a synthetic frame. This removes the sub-frame allocation and
interpreter dispatch from the common heap operations, while keeping exact
semantics. Heap-heavy loops also opt out of OSR (`CompiledChunk.LoopOSRable`),
because a loop that bridges every iteration is slower than the interpreter loop.

Object properties stay on the bridge. Introducing hidden shapes or dense slots
must first specify descriptor, accessor, prototype, `__proto__`, and GC-epoch
behavior; otherwise a stale cache would be a correctness bug.

Decisions recorded during implementation:

1. Default remains `--jit=auto` with the conservative eligibility check.
2. The bridge-exit model is the v1 foundation; direct calls are deferred except
   for pure numeric self-recursion.
3. `vm.Call` now returns an error; the bridge uses it for uncaught throws.
4. Object properties stay on the bridge until a handle-table/shape design is
   written (see §6.1).
5. `for...of` lowering and the string-builder fast path are deferred as
   independent optimizations.
6. The default code cache is 16 MiB, bounded per context and configurable.

## 8. Handoff checklist

- [x] Read this document and §1.1 first; reproduce at least two known
      miscompilations before changing code.
- [x] Run the baseline suite and benchmarks with JIT off and auto.
- [x] Start with Phase 0, not with opcode additions.
- [x] Keep the interpreter as the oracle: every new JIT feature needs a
      differential or unit test.
- [x] Never compile an operation whose semantics are not covered by a guard,
      bridge, or deopt.
- [x] Keep the "no Go calls while JIT frames are live" invariant in every
      review.
- [x] Follow repo workflow: one commit per fixed test case, short description.
      (Commits are grouped by roadmap phase; each phase passed the differential
      suite before its commit.)
- [x] Update this plan's checkboxes and open questions as decisions are made.

## 9. Handoff note (implementation result)

The amd64 JIT expansion described above is implemented. The interpreter remains
the oracle: every positive test can be run with `--jit=off` and `--jit=force`
and compared (`GO_JS_DIFFERENTIAL=1`).

Architecture as shipped:

- A shared `jit.Context` owns the chunk-keyed code cache, failure cache,
  backedge counters, blacklists, statistics, native-stack pool, and activation
  pool. Compilation and cache lookups are mutex-protected; execution of sealed
  code needs no lock.
- `jitEnter` switches from the Go goroutine stack to a dedicated mmap stack and
  loads the `JITActivation` into the reserved registers (`RBX`, `R12`-`R15`,
  `R11`). Compiled code never calls Go while a JIT frame is live. Every exit
  (done, deopt, bridge, throw, safepoint) returns to Go.
- Code is built in a Go byte buffer, copied into page-rounded RW mappings, then
  `mprotect`ed RX. There is no RWX mapping. The total arena is bounded by
  `--jit-max-code-bytes` (16 MiB default) and maps are reusable only through the
  context.
- The activation uses the interpreter's own frame layout (`locals + temporaries`
  in one array, top in `R11`). Deopt copies that array back to `vm.stack` and
  restores the frame's heap scope, so interpreter resume is exact.
- Numeric constants, locals, globals, arithmetic, comparisons, strict equality,
  truthiness, branches, loops, and returns are native. A failed type/tag guard
  deoptimizes or bridges before any stack mutation.
- Everything else is bridged on the real VM: the bytecode instruction is
  replayed with a cloned `ObjFunction` whose chunk holds the single instruction
  plus an internal `OP_JIT_HALT`, so `this`, arrow lexical `this`, heap scopes,
  method handles, constructors, iterators, property access, string coercion, and
  exceptions all use the interpreter's exact code path. Member operations,
  element pushes, and array/object creation use direct Go helpers instead of a
  replay frame.
- Pure numeric self-recursive chunks get direct machine `CALL`s with a
  guard-free body, an activation frame stride, a native-stack guard, and a
  JS-depth guard. All other calls bridge.
- OSR is attempted at backward branches. The whole chunk is compiled once; the
  loop header is just an alternate entry. `LoopOSRable` refuses loops whose
  bodies contain an always-bridged or always-deopted instruction. A loop budget
  (65,536 backedges) returns to Go for GC and `Gosched`.
- Deopt storms blacklist a chunk after 16 runtime deopts; compile failures are
  cached per chunk.

Flags:

| Flag | Meaning |
| --- | --- |
| `--jit=off` | Interpreter only; no compilation, no OSR. |
| `--jit=auto` | Default; compile after `--jit-call-threshold` calls and OSR after `--jit-loop-threshold` backedges. |
| `--jit=force` | Compile on first call; OSR uses the loop threshold. |
| `--jit-call-threshold` | Invocation counter threshold. |
| `--jit-loop-threshold` | Backedge counter threshold. |
| `--jit-max-code-bytes` | Bounded code arena (default 16 MiB). |
| `--jit-stats` | Print compile/deopt/bridge/OSR/safepoint counters on exit. |
| `--jit-trace` | Trace compilation, segments, bridges, and OSR entries. |

Verification:

- `runtime/jit` unit tests cover activation offsets, decoder fuzzing, direct
  analysis, encoder/arena limits, W^X page permissions, and concurrent compile.
- `test/test-scripts/jit-*.js` covers the §1.1 miscompiles, NaN comparisons,
  globals changing type, missing and extra arguments, zero-argument results,
  indirect calls, closures, methods, arrows, constructors, spread/rest, default
  parameters, mutual recursion, direct and indirect stack overflow, GC churn,
  OSR top-level/nested/break/continue/empty/unsupported loops, array/object
  mutation, strings, iterators, classes, `new`/`in`/`instanceof`, and hot
  callbacks.
- `GO_JS_DIFFERENTIAL=1 ./test.sh` runs the interpreter and forced JIT on every
  positive case and fails on stdout or diagnostic divergence. The final Docker
  run passed 506/506 tests plus the AOC puzzle and all benchmarks. A local run
  against Node 20 reports only the modern-feature probes that Node 20 itself
  cannot evaluate; the Docker Node 24 reference passes them.

Deferred, with reasons:

- Direct monomorphic calls and call-target inline caches. The bridge executes
  real `OP_CALL` on the same VM, so callees can themselves be JIT compiled and
  all call semantics stay in one place.
- Direct `ObjArr.items`/object-member machine-code fast paths and property
  caches. The heap's handle table is a Go map; see §6.1 for the design work
  required. Object access remains on the bridge, and heap-heavy loops remain
  interpreted via the OSR profitability check.
- `for...of` lowering and a string-builder fast path. Both are independent
  optimizations that can be added without changing the JIT ABI.
- No JIT frame metadata is emitted, so CPU profiling (`pprof`) must be run with
  `--jit=off`. Go async preemption can still interrupt a JIT segment; the JIT
  does not depend on it for correctness.

Measured on the development machine (Linux amd64, Go 1.26.3):

| Workload | `--jit=off` | `--jit=auto` |
| --- | --- | --- |
| recursive `fib(30)` | ~0.94 s | ~0.03 s |
| recursive `fib(40)` | ~97 s (extrapolated) | ~2.7 s |
| `sum(5_000_000)` loop in a one-shot function | ~1.1 s | ~0.15 s |
| top-level `for` loop, 5M numeric iterations | ~1.2 s | ~0.1 s |
| `benchmark-scripts/array-iteration.js` | ~30 s | ~30 s (kept interpreted by the OSR profitability rule) |
| `benchmark-scripts/object-access.js` | ~6.7 s | ~6.7 s (bridged) |
| `benchmark-scripts/string-operations.js` | ~5.4 s | ~5.4 s (bridged) |
| `benchmark-scripts/try-catch-shenanigans.js` | ~1.1 s | ~1.1 s (non-jittable, interpreter) |

The test harness allows 120 s for a benchmark because the interpreter's array
benchmark alone is close to 30 s on the development machine and can be slower
under CI load. Wall-clock timing
is still recorded rather than gated.
