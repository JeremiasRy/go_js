// Probe 22 — rejecting with an error passed through a helper callback.
//
// Node 24 baseline: prints "caught: boom".
//
// go_js observed (2026-11-14): prints "caught: Value is not callable".
// The rejection path executes, but the error object is replaced by a runtime
// TypeError from the VM's own internals.

function attempt(cb) {
  setTimeout(() => {
    cb(new Error('boom'));
  }, 10);
}

try {
  await new Promise((resolve, reject) => {
    attempt((e) => reject(e));
  });
} catch (e) {
  console.log('caught:', e.message);
}
