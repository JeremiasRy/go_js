// M2: Delete affects own configurable properties, not inherited values/length.
const parent = { inherited: 1 };
const object = Object.create(parent);
object.own = 2;
console.log("delete", delete object.own, "own" in object, delete object.absent, delete object.inherited, object.inherited);
Object.defineProperty(object, "fixed", { value: 3, configurable: false });
console.log("sloppy-fixed", delete object.fixed, object.fixed);
function strictDelete() { "use strict"; return delete object.fixed; }
try { strictDelete(); } catch (error) { console.log("strict-fixed", error.name); }
const array = [1, 2, 3];
console.log("array", delete array[1], array.length, 1 in array, array[1]);
let keys = 0;
console.log("key-once", delete object[(keys++, "absent")], keys);
console.log("optional-delete", delete null?.[keys++], keys);
