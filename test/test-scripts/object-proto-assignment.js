const p = { x: 1 };
const o = { __proto__: p };
console.log(o.x, Object.getPrototypeOf(o) === p);
o.__proto__ = { y: 2 };
console.log(o.y, Object.getPrototypeOf(o).y);
