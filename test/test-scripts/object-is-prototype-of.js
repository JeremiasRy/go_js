const p = {};
const o = Object.create(p);
console.log(p.isPrototypeOf(o), Object.prototype.isPrototypeOf(o), p.isPrototypeOf(p));
