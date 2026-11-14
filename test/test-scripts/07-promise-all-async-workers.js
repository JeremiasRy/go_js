// Probe 07 — Promise.all over async-function promises.
//
// Node 24 baseline: prints "Promise.all async: a,b,c" after ~10 ms.
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// A single direct `await w()` can work (see the early capability probes), but
// collecting async-function promises in an array and passing them to
// Promise.all does not.

async function w(v) {
  await new Promise((resolve) => setTimeout(resolve, 10));
  return v;
}

const result = await Promise.all([w('a'), w('b'), w('c')]);
console.log('Promise.all async:', result.join(','));
