// M3: Short-circuiting and method-specific handling of empty slots.
const array = [1, 2, 3, 4];
let trace = "";
console.log("some", array.some(value => { trace += value; return value === 2; }), trace);
trace = "";
console.log("every", array.every(value => { trace += value; return value < 3; }), trace);
trace = "";
console.log("find", array.find(value => { trace += value; return value > 2; }), trace);
console.log("empty", [].some(() => true), [].every(() => false));
const sparse = [, undefined, 3];
trace = "";
sparse.some((value, index) => { trace += index; return false; });
console.log("some-skips-holes", trace);
trace = "";
sparse.find((value, index) => { trace += `${index}:${value} `; return false; });
console.log("find-visits-holes", trace);
const context = { minimum: 3 };
console.log("receiver", array.every(function (value, index, owner) { return this === context && owner === array && value === index + 1; }, context));
console.log("find-index", array.findIndex(function (value) { return value >= this.minimum; }, context));
