// M3: Entry ordering, symbol keys, duplicate keys, and iterator closing.
const symbol = Symbol("key");
const object = { b: 2, 2: "two", 1: "one", a: 1, [symbol]: 9 };
for (const [key, value] of Object.entries(object)) { console.log("entry", key, value); }
const restored = Object.fromEntries(Object.entries(object));
console.log("roundtrip", Object.keys(restored).join(","), restored.b, restored[symbol]);
const values = Object.fromEntries([["x", 1], ["x", 2], [symbol, 3], ["__proto__", 4]]);
console.log("keys", values.x, values[symbol], Object.hasOwn(values, "__proto__"), values.__proto__, Object.getPrototypeOf(values) === Object.prototype);
let trace = "";
const entry = { get 0() { trace += "key "; return "k"; }, get 1() { trace += "value "; return 5; } };
console.log("access-order", Object.fromEntries([entry]).k, trace);
let closed = 0;
const invalid = { [Symbol.iterator]() { return { next() { return { value: 1, done: false }; }, return() { closed++; return {}; } }; } };
try { Object.fromEntries(invalid); } catch (error) { console.log("invalid-entry", error.name, closed); }
