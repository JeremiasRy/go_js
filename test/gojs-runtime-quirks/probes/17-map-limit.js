// Probe 17 — bounded-concurrency mapLimit built on all().
//
// Node 24 baseline: prints "mapLimit: 10,20,30,40".
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// This is the nested-concurrency primitive used inside the full queue lab.

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

async function mapLimit(items, limit, fn) {
  const out = new Array(items.length);
  let cursor = 0;
  async function run() {
    while (true) {
      const i = cursor++;
      if (i >= items.length) return;
      out[i] = await fn(items[i]);
    }
  }
  const runners = [];
  for (let i = 0; i < Math.min(limit, items.length); i++) runners.push(run());
  await all(runners);
  return out;
}

const result = await mapLimit([1, 2, 3, 4], 2, async (x) => {
  await sleep(10);
  return x * 10;
});
console.log('mapLimit:', result.join(','));
