// M1: Optional chains skip keys/arguments and stop at grouping boundaries.
let trace = "";
function base(value) { trace += "base "; return value; }
function key() { trace += "key "; return "child"; }
console.log("null", base(null)?.[key()].name, trace);
trace = "";
console.log("present", base({ child: { name: "ok" } })?.[key()].name, trace);
console.log("nested", ({ child: null }).child?.name, ({})?.missing?.name);
console.log("falsy", ({ value: 0 })?.value, ""?.length, false?.missing);
try { console.log((null?.child).name); }
catch (error) { console.log("group-boundary", error.name); }
try { console.log(({}).child.name); }
catch (error) { console.log("ordinary-access", error.name); }
let reads = 0;
const object = { get child() { reads++; return undefined; } };
console.log("getter-once", object.child?.name, reads);
