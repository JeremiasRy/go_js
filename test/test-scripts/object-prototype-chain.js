// M2: [[Prototype]] is independent from the ordinary property named prototype.
const parent = { inherited: 1 };
const child = Object.create(parent);
child.prototype = "ordinary property";
console.log("independent", child.inherited, child.prototype, Object.getPrototypeOf(child) === parent);
const other = { inherited: 2 };
console.log("set-prototype-return", Object.setPrototypeOf(child, other) === child, child.inherited);
child.inherited = 3;
console.log("shadow", child.inherited, other.inherited);
delete child.inherited;
console.log("unshadow", child.inherited);
const dictionary = Object.create(null);
dictionary.key = 4;
console.log("null-prototype", Object.getPrototypeOf(dictionary) === null, dictionary.toString, Object.keys(dictionary).join(","));
try { Object.setPrototypeOf(other, child); } catch (error) { console.log("cycle", error.name); }
Object.preventExtensions(child);
console.log("same-prototype", Object.setPrototypeOf(child, other) === child);
try { Object.setPrototypeOf(child, parent); } catch (error) { console.log("nonextensible", error.name); }
