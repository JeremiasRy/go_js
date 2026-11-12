let closed = 0;
const it = { [Symbol.iterator]() { return { next() { throw new RangeError("x"); }, return() { closed++; return { done: true }; } }; } };
try { const [a] = it; } catch (e) { console.log(e.name, closed); }
