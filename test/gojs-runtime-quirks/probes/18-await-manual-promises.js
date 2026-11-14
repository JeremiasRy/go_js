// Probe 18 — manual promises are the reliable async subset.
//
// Node 24 baseline:
//   one=42
//   two=7
//   counter=a,b,c
//
// go_js observed (2026-11-14): both constructs passed when tested separately.
// Timers, manual Promise constructors, multiple sequential awaits of those
// manual promises, and a completion counter all work.

const p = new Promise((resolve) => {
  setTimeout(() => resolve(42), 10);
});
console.log('one=' + (await p));

const q = new Promise((resolve) => {
  setTimeout(() => resolve(7), 10);
});
console.log('two=' + (await q));

let finish;
const done = new Promise((resolve) => {
  finish = resolve;
});
let n = 0;
const results = [];
setTimeout(() => {
  n++;
  results.push('a');
  if (n === 3) finish();
}, 10);
setTimeout(() => {
  n++;
  results.push('b');
  if (n === 3) finish();
}, 20);
setTimeout(() => {
  n++;
  results.push('c');
  if (n === 3) finish();
}, 30);
await done;
console.log('counter=' + results.join(','));
