// Probe 12 — await an async-function promise stored in a variable.
//
// Node 24 baseline: prints "await variable: a".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// Note: a single inline `await w()` can succeed; assigning the returned
// promise to a variable and awaiting that variable does not.

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function w() {
  await sleep(10);
  return 'a';
}

const p = w();
console.log('await variable:', await p);
