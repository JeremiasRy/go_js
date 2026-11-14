// Probe 27 — callback-style fan-out with fetch.
//
// Node 24 baseline:
//   fetch start
//   t1=1
//   t2=2
//   fetch done
//
// go_js observed (2026-11-14): the tool call timed out after 10 s. The
// script produced no output, even though simple fetch and simple timers each
// worked in isolation (probes 01-02). The heartbeat interval keeps the
// process alive in Node; in go_js the job never completed.

let pending = 2;
console.log('fetch start');

function fin() {
  pending--;
  if (pending === 0) {
    console.log('fetch done');
    clearInterval(hb);
  }
}

fetch('https://jsonplaceholder.typicode.com/todos/1')
  .then((res) => res.json())
  .then(
    (d) => {
      console.log('t1=' + d.id);
      fin();
    },
    (e) => {
      console.log('e1=' + e.message);
      fin();
    }
  );

fetch('https://jsonplaceholder.typicode.com/todos/2')
  .then((res) => res.json())
  .then(
    (d) => {
      console.log('t2=' + d.id);
      fin();
    },
    (e) => {
      console.log('e2=' + e.message);
      fin();
    }
  );

const hb = setInterval(() => {}, 20);
