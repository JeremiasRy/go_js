// M2: Nested patterns only default undefined and evaluate sources/keys once.
let trace = "";
function source() { trace += "source "; return { rows: [{ value: undefined }], extra: 3 }; }
function key() { trace += "key "; return "rows"; }
function fallback() { trace += "default "; return 7; }
const { [key()]: [{ value: renamed = fallback() }], ...remaining } = source();
console.log("nested", renamed, remaining.extra, trace);
const { a = 1, b = 2 } = { a: null, b: undefined };
const [first = 3, , third = 5] = [undefined, 4];
console.log("defaults", a, b, first, third);
const { inherited } = Object.create({ inherited: 9 });
console.log("inherited-read", inherited);
try { const { nested: { value } } = { nested: null }; console.log(value); }
catch (error) { console.log("null-nested", error.name); }
const { length } = "abc";
console.log("boxed-source", length);
