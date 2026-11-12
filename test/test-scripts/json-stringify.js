// M3: Omission/null rules, ordering, hooks, formatting, and cycle errors.
const value = { b: undefined, a: 1, fn() {}, items: [undefined, function () {}, NaN, Infinity, -0] };
console.log("omission", JSON.stringify(value), JSON.stringify(undefined), JSON.stringify(function () {}));
console.log("key-order", JSON.stringify({ b: 2, 10: 10, 2: 2, a: 1 }));
console.log("escaping", JSON.stringify("a\n\"\\\uD800"));
let trace = "";
const object = { child: { toJSON(key) { trace += "toJSON:" + key + " "; return { number: 2 }; } }, omit: 3 };
const text = JSON.stringify(object, function (key, item) {
    trace += "replace:" + (key || "root") + " ";
    if (key === "omit") return undefined;
    return typeof item === "number" ? item * 10 : item;
});
console.log("hooks", text, trace);
console.log("property-list", JSON.stringify({ a: 1, b: 2, c: 3 }, ["c", "a", "c"]));
console.log("spacing", JSON.stringify({ a: 1 }, null, 2).split("\n").join("|"));
console.log("spacing-cap", JSON.stringify({ a: 1 }, null, "abcdefghijkl").split("\n")[1].startsWith("abcdefghij\""));
const shared = { a: 1 };
console.log("shared-reference", JSON.stringify([shared, shared]));
const cycle = {};
cycle.self = cycle;
try { JSON.stringify(cycle); } catch (error) { console.log("cycle", error.name); }
