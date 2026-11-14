# nested-function-declaration-queuemicrotask.js

Node 24 baseline (`v24.22.2`):

```text
start
sync end
microtask
other timer
```

Current go_js behavior:

```text
stdout:
start
sync end

stderr:
(empty)

exit: killed by timeout, no further output
```

**Failure.** `queueMicrotask` accepts the nested function declaration, but the microtask never runs. The process then stalls: the 40 ms timer does not fire and the runtime never exits. This is a hang, not a thrown error. The same callback as a nested function expression or arrow runs normally, and a top-level declaration works too.

**Not covered.** There is no diagnostic or error class to assert. The repro needs an external harness timeout. It does not establish whether other queued microtasks would run after the bad entry, because the whole event loop stalls.
