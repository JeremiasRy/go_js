// Probe 16 — a hand-rolled Promise.all over async-function promises.
//
// Node 24 baseline: prints "custom all async: a,b,c".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// Replacing Promise.all with an explicit .then-based implementation does not
// help; the failure follows async-function promises into any combinator.

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function all(promises) {
  return new Promise((resolve, reject) => {
    let n = promises.length;
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

async function w(v) {
  await sleep(10);
  return v;
}

const result = await all([w('a'), w('b'), w('c')]);
console.log('custom all async:', result.join(','));
