// Probe 27 — callback-style fan-out with fetch.
//
// Node 24 baseline: both requests complete and the fan-out drains.
// Completion order is normalized so the output is deterministic.

let pending = 2;
const ids = [];
console.log('fetch start');

function fin() {
  pending--;
  if (pending === 0) {
    console.log('ids=' + ids.sort(function (a, b) { return a - b; }).join(','));
    console.log('fetch done');
    clearInterval(hb);
  }
}

fetch('https://jsonplaceholder.typicode.com/todos/1')
  .then((res) => res.json())
  .then(
    (d) => {
      ids.push(d.id);
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
      ids.push(d.id);
      fin();
    },
    (e) => {
      console.log('e2=' + e.message);
      fin();
    }
  );

const hb = setInterval(() => {}, 20);
