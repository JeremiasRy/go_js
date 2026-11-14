// Probe 31 — an async function returning a manually constructed promise.
//
// Node 24 baseline: prints "async manual: x".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// Even wrapping the manual promise in an async function makes the result
// unusable by the caller.

async function w() {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve('x');
    }, 10);
  });
}

const v = await w();
console.log('async manual:', v);
