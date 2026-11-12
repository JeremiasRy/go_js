// M1: Each lexical loop iteration has its own captured bindings.
const readers = [];
for (let index = 0; index < 3; index++) { readers.push(() => index); }
console.log("for", readers[0](), readers[1](), readers[2]());
const values = [];
for (const value of [4, 5, 6]) { values.push(() => value); }
console.log("for-of", values[0](), values[1](), values[2]());
const keys = [];
for (let key in { a: 1, b: 2 }) { keys.push(() => key); }
console.log("for-in", keys[0](), keys[1]());
const pairs = [];
for (let index = 0; index < 2; index++) {
    pairs.push([() => ++index, () => index]);
}
console.log("shared-within-iteration", pairs[0][0](), pairs[0][1](), pairs[1][1]());
console.log("outside", typeof index, typeof key);
