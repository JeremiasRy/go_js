// M3: Clamped/negative indices, sparse copies, and omitted arguments.
const original = [0, 1, 2, 3, 4];
console.log("slice", original.slice(1, 3).join(","), original.slice(-3, -1).join(","), original.slice(-99, 99).join(","));
console.log("slice-empty", original.slice(3, 1).length, original.slice(Infinity).length);
const copy = original.slice();
copy[0] = 99;
console.log("copy", copy !== original, original[0], copy[0]);
const sparse = [, 1, , 3];
const sparseCopy = sparse.slice(0, 3);
console.log("sparse-copy", sparseCopy.length, 0 in sparseCopy, 1 in sparseCopy, 2 in sparseCopy);
const edited = [0, 1, 2, 3];
console.log("splice", edited.splice(1, 2, "a", "b", "c").join(","), edited.join(","));
console.log("negative-start", edited.splice(-2, 1).join(","), edited.join(","));
console.log("omitted", edited.splice().length, edited.splice(1, undefined, 9).length, edited.join(","));
console.log("delete-rest", edited.splice(2).join(","), edited.join(","));
const removed = sparse.splice(0, 2);
console.log("sparse-splice", removed.length, 0 in removed, 1 in removed, sparse.length, 0 in sparse);
