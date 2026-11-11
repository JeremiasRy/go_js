// M2: Explicit receivers, array-like apply, binding, and bound construction.
function sum(a, b) { "use strict"; return this.base + a + b; }
const receiver = { base: 10 };
console.log("call", sum.call(receiver, 1, 2));
console.log("apply", sum.apply(receiver, { 0: 3, 1: 4, length: 2 }));
let trace = "";
const arrayLike = { length: 2, get 0() { trace += "0"; return 5; }, get 1() { trace += "1"; return 6; } };
console.log("apply-order", sum.apply(receiver, arrayLike), trace);
const bound = sum.bind(receiver, 7);
console.log("bound", bound(8), bound.call({ base: 100 }, 8), bound.bind({ base: 200 })(8));
console.log("metadata", bound.length, bound.name, Object.hasOwn(bound, "prototype"));
function count() { return arguments.length; }
console.log("empty-apply", count.apply(null, null), count.apply(null, undefined));
try { count.apply(null, 3); } catch (error) { console.log("invalid-apply", error.name); }
function Item(a, b) { this.total = a + b; }
const BoundItem = Item.bind({ ignored: true }, 4);
const item = new BoundItem(5);
console.log("bound-new", item.total, item instanceof Item, item instanceof BoundItem);
