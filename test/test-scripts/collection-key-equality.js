// M3: SameValueZero, string value equality, object identity, and symbol keys.
const map = new Map();
map.set(NaN, "first").set(0 / 0, "second").set(-0, "zero");
console.log("numeric-map", map.size, map.get(NaN), map.get(0), map.has(-0));
map.set("same", 1).set(["sa", "me"].join(""), 2);
console.log("string-map", map.size, map.get("same"));
const object = {};
map.set(object, "object");
console.log("object-map", map.get(object), map.has({}));
const symbol = Symbol("key");
map.set(symbol, "symbol");
console.log("symbol-map", map.get(symbol), map.has(Symbol("key")));
const set = new Set([NaN, 0 / 0, -0, 0, "same", "sa" + "me", object, object, {}]);
console.log("set-equality", set.size, set.has(NaN), set.has(0), set.has("same"), set.has(object), set.has({}));
const zeros = new Set([-0]);
console.log("canonical-zero", Object.is([...zeros][0], 0), Object.is([...new Map([[-0, 1]]).keys()][0], 0));
