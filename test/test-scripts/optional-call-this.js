const o = { x: 1, m() { return this === undefined ? "undefined" : typeof this; } };
const m = o.m;
console.log(m());
console.log(m?.());
