// Probe 14 — two sequential awaits of async-function calls.
//
// Node 24 baseline: prints "await twice: ab".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// Even without storing promises, a second async call in the same script can
// fail. Async-function support is effectively single-use.

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function w(v) {
  await sleep(10);
  return v;
}

const a = await w('a');
const b = await w('b');
console.log('await twice:', a + b);
