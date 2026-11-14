// Probe 23 — error callback that logs and resolves a literal.
//
// Node 24 baseline:
//   err: boom
//   resolved: done
//
// go_js observed (2026-11-14): identical behavior (it printed the error and
// then resolved). This shows the same helper/timer shape as probes 21-22 can
// work when `resolve` is called with a literal rather than forwarded data.

function attempt(cb) {
  setTimeout(() => {
    cb(new Error('boom'));
  }, 10);
}

await new Promise((resolve) => {
  attempt((e) => {
    console.log('err:', e.message);
    resolve('done');
  });
});

console.log('resolved: done');
