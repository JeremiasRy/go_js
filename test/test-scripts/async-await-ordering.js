async function a() { await null; console.log('a1'); await null; console.log('a2'); }
async function b() { await null; console.log('b1'); await null; console.log('b2'); }
a();
b();
Promise.resolve().then(() => console.log('p1')).then(() => console.log('p2'));
console.log('sync');
