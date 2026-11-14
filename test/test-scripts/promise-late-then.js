let resolve;
const pending = new Promise((r) => { resolve = r; });
setTimeout(() => resolve('late'), 20);

console.log('value', await pending);
console.log('then', await pending.then((v) => v + '!'));
const a = pending.then((v) => v + '1');
const b = pending.then((v) => v + '2');
console.log('late then', await a, await b);
