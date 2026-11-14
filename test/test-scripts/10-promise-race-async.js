// Probe 10 — Promise.race where an async-function promise wins.
//
// Node 24 baseline: prints "race: a" after ~10 ms.
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// If the timeout rejects first instead, the same race shape succeeds — see
// probe 11. The crash happens when the runtime must settle the race with the
// async function's own promise.

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function w() {
  await sleep(10);
  return 'a';
}

const timeout = new Promise((_, reject) => {
  setTimeout(() => reject(new Error('timeout')), 100);
});

console.log('race:', await Promise.race([w(), timeout]));
