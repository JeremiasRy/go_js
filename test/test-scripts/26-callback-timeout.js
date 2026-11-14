// Probe 26 — callback-only timeout around a slow task.
//
// Node 24 baseline:
//   TIMEOUT fired
//   result=ETIMEOUT
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable

function runWithTimeout(ms, fn, done) {
  let settled = false;
  const timer = setTimeout(() => {
    if (settled) return;
    settled = true;
    console.log('TIMEOUT fired');
    done(new Error('ETIMEOUT'));
  }, ms);

  fn((err, value) => {
    if (settled) return;
    settled = true;
    clearTimeout(timer);
    done(err, value);
  });
}

runWithTimeout(
  50,
  (cb) => {
    setTimeout(() => {
      console.log('task finished (too late)');
      cb(null, 'ok');
    }, 100);
  },
  (e, v) => {
    console.log('result=' + (e ? e.message : v));
    clearInterval(hb);
  }
);

const hb = setInterval(() => {}, 20);
