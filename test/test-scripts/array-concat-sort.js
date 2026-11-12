// M3: Concat spreading and sort stability/default ordering.
const first = [1, , 3];
const combined = first.concat([4], 5);
console.log("concat", combined.join(","), combined.length, 1 in combined, first.length);
const arrayLike = { 0: "a", 1: "b", length: 2, [Symbol.isConcatSpreadable]: true };
console.log("spreadable", [0].concat(arrayLike).join(","));
const nested = [6, 7];
nested[Symbol.isConcatSpreadable] = false;
console.log("unspreadable", [].concat(nested).length, [].concat(nested)[0] === nested);
const values = [10, 2, 1];
console.log("default-sort", values.sort() === values, values.join(","));
console.log("numeric-sort", values.sort((a, b) => a - b).join(","));
const records = [{ key: 2, id: "a" }, { key: 1, id: "b" }, { key: 2, id: "c" }, { key: 1, id: "d" }];
console.log("stable", records.sort((a, b) => a.key - b.key).map(value => value.id).join(","));
const sparse = [, undefined, 3, 1];
sparse.sort();
console.log("undefined-before-hole", sparse[0], sparse[1], 2 in sparse, sparse[2], 3 in sparse);
try { [2, 1].sort(() => { throw new Error("compare"); }); }
catch (error) { console.log("comparator-error", error.message); }
