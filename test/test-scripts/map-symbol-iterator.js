const m = new Map([["a", 1]]);
console.log(typeof m[Symbol.iterator]);
try { console.log(m[Symbol.iterator]().next().value.join(":")); } catch (e) { console.log(e.name); }
