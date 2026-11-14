// Probe 13 — await an async-function promise stored in an array.
//
// Node 24 baseline: prints "await array element: a".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function w() {
  await sleep(10);
  return 'a';
}

const arr = [w()];
console.log('await array element:', await arr[0]);
