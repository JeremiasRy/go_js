// M3: Callback arguments, sparse traversal, initial lengths, and reduction.
const input = [1, , 3];
const context = { factor: 2 };
let trace = "";
const mapped = input.map(function (value, index, owner) {
    trace += `${index}:${owner === input}:${this === context} `;
    return value * this.factor;
}, context);
console.log("map", mapped.join(","), mapped.length, 1 in mapped, trace);
console.log("filter", input.filter(value => -value).join(","));
const changing = [1, 2, 3];
trace = "";
const returned = changing.forEach((value, index, owner) => {
    trace += `${index}:${value} `;
    if (index === 0) { owner.push(4); delete owner[1]; owner[2] = 9; }
});
console.log("snapshot-length", trace, returned, changing.length);
console.log("reduce", input.reduce((sum, value, index, owner) => sum + value + index + (owner === input ? 0 : 100), 0));
console.log("single-reduce", [, 5].reduce((a, b) => a + b));
console.log("explicit-undefined", [].reduce((a, b) => a + b, undefined));
for (const empty of [[], new Array(2)]) {
    try { empty.reduce((a, b) => a + b); } catch (error) { console.log("empty-reduce", error.name); }
}
const inherited = [1, , 3];
const prototype = Object.create(Array.prototype);
prototype[1] = 2;
Object.setPrototypeOf(inherited, prototype);
console.log("inherited-index", inherited.map(value => value * 2).join(","));
try { input.map(() => { throw new Error("callback"); }); }
catch (error) { console.log("callback-error", error.message); }
