// M1: != is the negation of Abstract Equality, including coercion.
for (const pair of [[1, "1"], [0, false], [null, undefined], [1, 2], ["", 0], [NaN, NaN], [null, 0]]) {
    console.log("not-equal", pair[0] != pair[1], pair[0] == pair[1]);
}
const object = {};
console.log("objects", object != object, object != {}, [] != []);
let calls = 0;
const numeric = { valueOf() { calls++; return 3; } };
console.log("coerce", numeric != 3, 3 != numeric, numeric !== 3, calls);
