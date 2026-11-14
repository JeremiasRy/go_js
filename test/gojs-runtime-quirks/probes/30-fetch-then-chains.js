// Probe 30 — fetch().then() chains.
//
// Node 24 baseline:
//   fetch nested: delectus aut autem
//   fetch return: quis ut nam facilis et officia qui
//
// go_js observed (2026-11-14): both forms passed. Native fetch promises and
// .then chaining work when the chain is kicked off inside a manual promise
// that is awaited once.
//
// This is the most useful network workaround discovered.

async function nestedForm() {
  return new Promise((resolve, reject) => {
    fetch('https://jsonplaceholder.typicode.com/todos/1').then(
      (res) => {
        res.json().then(
          (data) => resolve(data.title),
          (e) => reject(e)
        );
      },
      (e) => reject(e)
    );
  });
}

async function returnForm() {
  return new Promise((resolve, reject) => {
    fetch('https://jsonplaceholder.typicode.com/todos/2')
      .then((res) => res.json())
      .then(
        (data) => resolve(data.title),
        (e) => reject(e)
      );
  });
}

console.log('fetch nested:', await nestedForm());
console.log('fetch return:', await returnForm());
