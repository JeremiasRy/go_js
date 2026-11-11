// M3: Map construction, iteration order, mutations, and size accessors.
const map = new Map([["a", 1], ["b", 2], ["a", 3]]);
console.log("construct", map.size, map.get("a"), map.set("c", 4) === map);
console.log("iterators", [...map.keys()].join(","), [...map.values()].join(","));
for (const [key, value] of map) { console.log("entry", key, value); }
console.log("delete", map.delete("b"), map.delete("missing"), map.has("b"), map.get("missing"));
map.set("b", 5);
console.log("reinsert", [...map.keys()].join(","));
const context = {};
map.forEach(function (value, key, owner) { console.log("foreach", key, value, owner === map, this === context); }, context);
const changing = new Map([[1, "one"], [2, "two"]]);
const iterator = changing.entries();
console.log("first", iterator.next().value.join(":"));
changing.delete(2);
changing.set(3, "three");
console.log("live", iterator.next().value.join(":"), iterator.next().done);
function writeSize() { "use strict"; map.size = 0; }
try { writeSize(); } catch (error) { console.log("size-readonly", error.name); }
console.log("clear", map.clear(), map.size, [...map].length);
try { new Map([1]); } catch (error) { console.log("invalid-entry", error.name); }
