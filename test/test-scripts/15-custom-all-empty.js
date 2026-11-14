// Probe 15 — hand-rolled Promise.all with an empty array.
//
// Node 24 baseline: prints "custom all empty: 0".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// The executor resolves synchronously with an empty array, so this also
// exposes problems with promise construction outside timers.

function all(promises) {
  return new Promise((resolve, reject) => {
    const n = promises.length;
    const out = new Array(n);
    if (n === 0) {
      resolve(out);
      return;
    }
    promises.forEach((p, i) => {
      p.then(
        (v) => {
          out[i] = v;
          n--;
          if (n === 0) resolve(out);
        },
        (e) => reject(e)
      );
    });
  });
}

const result = await all([]);
console.log('custom all empty:', result.length);
