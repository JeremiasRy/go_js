async function waiter(name, p) {
  const value = await p;
  console.log(name, value);
  return value;
}

let resolve;
const shared = new Promise((r) => { resolve = r; });
const first = waiter('first', shared);
const second = waiter('second', shared);
setTimeout(() => resolve(42), 10);

const values = await Promise.all([first, second]);
console.log('values', values.join(','));
