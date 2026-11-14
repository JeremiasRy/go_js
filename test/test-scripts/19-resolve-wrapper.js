// Probe 19 — resolving a manual promise through a stored wrapper function.
//
// Node 24 baseline:
//   wrapper
//   A=x
//
// go_js observed (2026-11-14): identical output. Storing a function that
// closes over `resolve` in a variable works, as long as it is invoked
// directly from the executor's timer.

let finish;
const done = new Promise((resolve) => {
  finish = (v) => {
    console.log('wrapper');
    resolve(v);
  };
});

setTimeout(() => {
  finish('x');
}, 20);

const v = await done;
console.log('A=' + v);
