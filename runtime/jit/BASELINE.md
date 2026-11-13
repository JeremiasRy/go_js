# JIT Baseline Timings

Recorded at the start of the JIT expansion work on Linux amd64, Go 1.26.3.
Wall-clock total process time, `--jit=off` vs `--jit=auto` (the old
proof-of-concept JIT only compiles direct self-recursion).

| Workload | JIT off | JIT auto (old JIT) |
| --- | --- | --- |
| `fib(30)` recursive | 813 ms | 19 ms |
| `sum(5_000_000)` numeric loop in a function called once | 1123 ms | 1097 ms |
| `benchmark-scripts/array-iteration.js` | 25787 ms | not measured |
| `benchmark-scripts/object-access.js` | 7231 ms | not measured |
| `benchmark-scripts/string-operations.js` | 5182 ms | not measured |

The numeric loop and benchmark scripts are not accelerated by the old JIT:
the loop lives in a function or top-level body that is only invoked once, so
the call-count trigger never fires. This is the motivation for OSR.
