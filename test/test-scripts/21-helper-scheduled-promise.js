// Probe 21 — scheduling a promise resolution through a helper callback.
//
// Node 24 baseline: prints "helper: z".
//
// go_js observed (2026-11-14): the job reports success (exit 0) but produces
// no output at all. The timer callback that calls `cb('z')` either never runs
// or its result is lost.
//
// Compare probe 19, where the wrapper is invoked directly from a timer inside
// the executor, and probe 23, where the callback logs and resolves a literal.

function attempt(cb) {
  setTimeout(() => {
    cb('z');
  }, 10);
}

const v = await new Promise((resolve) => {
  attempt((x) => resolve(x));
});

console.log('helper:', v);
