// Probe 24 — racing an async function wrapped in a manual promise.
//
// Node 24 baseline: prints "wrapper race: a".
//
// go_js observed (2026-11-14): exits 0 and prints nothing. The wrapper's
// async IIFE never publishes a result (same silent failure class as probe 21).
//
// Variants `wrapper.then(...)` and `Promise.all([wrapped(...), wrapped(...)])`
// also completed with empty output.

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function wrapped(fn) {
  return new Promise((resolve, reject) => {
    (async () => {
      try {
        resolve(await fn());
      } catch (e) {
        reject(e);
      }
    })();
  });
}

async function w() {
  await sleep(10);
  return 'a';
}

const v = await Promise.race([
  wrapped(w),
  new Promise((_, reject) => setTimeout(() => reject(new Error('timeout')), 100))
]);

console.log('wrapper race:', v);
