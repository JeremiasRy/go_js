// Probe 08 — .then() on an async-function promise.
//
// Node 24 baseline:
//   then: a
//   done waiting
//
// go_js observed (2026-11-14): exits 0 but prints only "done waiting".
// The fulfillment callback passed to .then() never runs and no error is
// reported. This is a silent no-op, not a crash.

async function w() {
  return 'a';
}

const p = w();
p.then(
  (v) => console.log('then:', v),
  (e) => console.log('error:', e.message)
);

await new Promise((resolve) => setTimeout(resolve, 50));
console.log('done waiting');
