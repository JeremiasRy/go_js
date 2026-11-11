// M3: Holes, canonical indices, length changes, and array property invariants.
const array = [, undefined, 3];
console.log("holes", array.length, 0 in array, 1 in array, array[0], array[99]);
array.length = 5;
console.log("grow", array.length, 3 in array, 4 in array);
array[6] = 7;
array["01"] = 8;
array[-1] = 9;
array[1.5] = 10;
array[4294967295] = 11;
console.log("indices", array.length, array["6"], array["01"], array[-1], array[1.5], array[4294967295]);
array.length = 2;
console.log("truncate", array.length, 2 in array, 6 in array, array["01"]);
for (const invalid of [-1, 1.5, 4294967296, NaN]) {
    try { array.length = invalid; } catch (error) { console.log("invalid-length", error.name); }
}
const blocked = [0, 1, 2, 3];
Object.defineProperty(blocked, "2", { configurable: false });
blocked.length = 1;
console.log("partial-truncation", blocked.length, 2 in blocked, 3 in blocked);
Object.defineProperty(blocked, "length", { writable: false });
function extend() { "use strict"; blocked[3] = 4; }
try { extend(); } catch (error) { console.log("readonly-length", error.name, blocked.length); }
