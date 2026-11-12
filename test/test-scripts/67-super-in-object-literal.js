const proto = { m() { return 1; } };
const o = { __proto__: proto, m() { return super.m() + 1; } };
console.log(o.m());
