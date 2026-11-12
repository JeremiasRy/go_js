// M3: ES2023 copy methods and reverse searches.
const array = [3, 1, 2];
console.log("copies", array.toSorted((a, b) => a - b).join(","), array.toReversed().join(","), array.toSpliced(1, 1, 9, 8).join(","), array.with(-1, 7).join(","));
console.log("original", array.join(","), array.toSorted() !== array);
console.log("spliced-arguments", array.toSpliced().join(","), array.toSpliced(1).join(","), array.toSpliced(1, undefined, 9).join(","));
const sparse = [, 2, , 1];
for (const copy of [sparse.toReversed(), sparse.toSorted(), sparse.toSpliced(1, 1), sparse.with(1, 9)]) {
    console.log("dense", copy.length, Object.keys(copy).length, copy.join(","));
}
for (const index of [3, -4, Infinity]) {
    try { array.with(index, 0); } catch (error) { console.log("with-bounds", error.name); }
}
let trace = "";
console.log("find-last", sparse.findLast((value, index) => { trace += index; return value === undefined; }), trace);
trace = "";
console.log("find-last-index", array.findLastIndex((value, index) => { trace += index; return value < 3; }), trace);
console.log("empty", [].findLast(() => true), [].findLastIndex(() => true));
