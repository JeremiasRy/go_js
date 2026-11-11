// M3: Set ordering, constructor iteration, chainability, and live traversal.
const set = new Set([3, 1, 3, 2]);
console.log("construct", set.size, [...set].join(","), set.add(4) === set);
console.log("keys-values", [...set.keys()].join(","), [...set.values()].join(","));
for (const [key, value] of set.entries()) { console.log("entry", key, value); }
console.log("delete", set.delete(1), set.delete(9), set.has(1));
set.add(1);
console.log("reinsert", [...set].join(","));
const changing = new Set([1, 2]);
const context = {};
changing.forEach(function (value, key, owner) {
    console.log("foreach", value, key, owner === changing, this === context);
    if (value === 1) { changing.delete(2); changing.add(3); }
}, context);
const iterator = changing.values();
console.log("first", iterator.next().value);
changing.add(4);
console.log("live", iterator.next().value, iterator.next().value, iterator.next().done);
console.log("string-input", [...new Set("a😀a")].join("|"));
function writeSize() { "use strict"; set.size = 0; }
try { writeSize(); } catch (error) { console.log("size-readonly", error.name); }
console.log("clear", set.clear(), set.size);
