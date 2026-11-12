// M3: Array construction, iterable/array-like from, closing, and species.
console.log("constructors", Array(3).length, 0 in Array(3), Array("3")[0], Array.of(3).length, Array.of(3)[0]);
console.log("is-array", Array.isArray([]), Array.isArray({ length: 0 }), Array.isArray(Array.prototype));
console.log("array-like", Array.from({ 0: "a", 2: "c", length: 3 }).join(","), 1 in Array.from({ length: 2 }));
const context = { offset: 10 };
console.log("mapping", Array.from([1, 2], function (value, index) { return this.offset + value + index; }, context).join(","));
console.log("unicode", Array.from("A😀B").join("|"));
let closed = 0;
const iterable = { [Symbol.iterator]() { return { next() { return { value: 1, done: false }; }, return() { closed++; return {}; } }; } };
try { Array.from(iterable, () => { throw new Error("map"); }); }
catch (error) { console.log("close-on-map-error", error.message, closed); }
function Species(length) { this.initialLength = length; }
const array = [2, 3];
array.constructor = { [Symbol.species]: Species };
const mapped = array.map(value => value * 2);
console.log("species", mapped instanceof Species, mapped.initialLength, mapped[0], mapped[1], Array.isArray(mapped));
try { Array.from(null); } catch (error) { console.log("null-source", error.name); }
