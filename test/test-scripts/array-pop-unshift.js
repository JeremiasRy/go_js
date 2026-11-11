// M3: Mutation order, return values, empty arrays, and generic receivers.
const array = [2, 3];
console.log("unshift", array.unshift(0, 1), array.join(","));
console.log("pop", array.pop(), array.join(","), array.length);
console.log("no-items", array.unshift(), array.length);
const empty = [];
console.log("empty", empty.pop(), empty.length, empty.unshift(), empty.unshift(1), empty.pop());
const sparse = [, 2];
console.log("sparse-unshift", sparse.unshift(0), 0 in sparse, 1 in sparse, 2 in sparse, sparse[2]);
sparse.length = 5;
console.log("sparse-pop", sparse.pop(), sparse.length, 3 in sparse);
const arrayLike = { 0: "a", length: 1 };
console.log("generic-unshift", Array.prototype.unshift.call(arrayLike, "b", "c"), arrayLike[0], arrayLike[1], arrayLike[2]);
console.log("generic-pop", Array.prototype.pop.call(arrayLike), arrayLike.length, 2 in arrayLike);
