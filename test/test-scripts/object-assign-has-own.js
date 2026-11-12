// M3: Assignment copies enumerable own properties through ordinary setters.
let trace = "";
const symbol = Symbol("key");
const source = Object.create({ inherited: 1 });
source[symbol] = 2;
Object.defineProperty(source, "hidden", { value: 3 });
Object.defineProperty(source, "value", { enumerable: true, get() { trace += "get "; return 4; } });
const target = { set value(next) { trace += "set:" + next + " "; } };
console.log("identity", Object.assign(target, null, undefined, source) === target);
console.log("copy", target[symbol], target.hidden, target.inherited, trace);
console.log("overwrite", Object.assign({}, { x: 1 }, { x: 2 }).x);
const dictionary = Object.create(null);
dictionary.hasOwnProperty = "shadowed";
dictionary[symbol] = 7;
console.log("has-own", Object.hasOwn(dictionary, "hasOwnProperty"), Object.hasOwn(dictionary, symbol), Object.hasOwn(dictionary, "missing"), Object.hasOwn(source, "inherited"));
const blocked = {};
Object.defineProperty(blocked, "locked", { value: 1 });
try { Object.assign(blocked, { before: 2, locked: 3, after: 4 }); }
catch (error) { console.log("partial-copy", error.name, blocked.before, blocked.locked, blocked.after); }
try { Object.assign(null, {}); } catch (error) { console.log("null-target", error.name); }
