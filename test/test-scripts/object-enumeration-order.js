// M2: Integer-index ordering, insertion order, symbols, and inherited shadowing.
const symbol = Symbol("key");
const object = { b: 1, 10: 10, 2: 2, "01": 1, a: 3, prototype: 4, [symbol]: 5 };
console.log("keys", Object.keys(object).join(","));
object.b = 9;
delete object.a;
object.a = 8;
console.log("reinsert", Object.keys(object).join(","), Object.values(object).join(","));
const parent = { shared: 1, inherited: 2, hiddenByOwn: 3 };
const child = Object.create(parent);
child.shared = 4;
child.own = 5;
Object.defineProperty(child, "hiddenByOwn", { value: 6, enumerable: false });
let keys = "";
for (const key in child) { keys += key + ","; }
console.log("for-in", keys);
console.log("own-names", Object.getOwnPropertyNames(child).join(","), Object.getOwnPropertySymbols(object)[0] === symbol);
