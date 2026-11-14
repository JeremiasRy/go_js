// Probe 25 — callback-only worker pool.
//
// Node 24 baseline:
//   start
//   worker 1
//   worker 2
//   worker 3
//   ALL DONE
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// This avoids promises entirely; it still fails. A heartbeat interval is used
// so the script stays alive until all callbacks have run.

let doneCount = 0;
const total = 3;

console.log('start');

function worker(id) {
  setTimeout(() => {
    console.log('worker ' + id);
    doneCount++;
    if (doneCount === total) {
      console.log('ALL DONE');
      clearInterval(hb);
    }
  }, 30 + id * 20);
}

worker(1);
worker(2);
worker(3);

const hb = setInterval(() => {}, 20);
