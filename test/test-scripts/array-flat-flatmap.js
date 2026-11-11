// M3: Flattening depth, hole removal, and flatMap callback semantics.
const nested = [1, [2, [3]], , [4, , 5]];
console.log("default-depth", nested.flat().length, Array.isArray(nested.flat()[2]), nested.flat().join("|"));
console.log("deep", nested.flat(Infinity).join(","));
console.log("zero-depth", nested.flat(0).length, Array.isArray(nested.flat(0)[1]), nested.length);
console.log("coerced-depth", nested.flat("2").join(","), nested.flat(-1).length);
let trace = "";
const input = [1, , 3];
const context = { factor: 2 };
const result = input.flatMap(function (value, index, owner) {
    trace += `${index}:${owner === input} `;
    return [, value * this.factor];
}, context);
console.log("flatmap", result.join(","), trace);
console.log("one-level", Array.isArray([1].flatMap(value => [[value]])[0]));
console.log("array-like-kept", [1].flatMap(() => ({ 0: "a", length: 1 }))[0].length);
