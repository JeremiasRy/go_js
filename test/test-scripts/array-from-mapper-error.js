try { Array.from([1], () => { throw new RangeError("x"); }); } catch (e) { console.log(e.name); }
let closed = 0;
const it = { [Symbol.iterator]() { let i = 0; return { next() { return { value: i++, done: i > 2 }; }, return() { closed++; return { done: true }; } }; } };
try { Array.from(it, () => { throw new RangeError("x"); }); } catch (e) { console.log(e.name, closed); }
