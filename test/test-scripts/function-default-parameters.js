// M2: Defaults, parameter environments, arguments, and function length.
let calls = 0;
function defaults(a = ++calls, b = a + 1) { return `${a}:${b}`; }
console.log("defaults", defaults(), defaults(undefined, 8), defaults(null), defaults(0), calls);
console.log("length", defaults.length, ((a, b = 2, c) => c).length);
const outside = "outer";
function scope(value = outside) { const outside = "body"; return value + ":" + outside; }
console.log("parameter-scope", scope());
function later(a = b, b = 2) { return a + b; }
try { later(); } catch (error) { console.log("later-tdz", error.name); }
function self(value = value) { return value; }
try { self(); } catch (error) { console.log("self-tdz", error.name); }
function fresh(value = []) { value.push(1); return value.length; }
console.log("per-call", fresh(), fresh());
function unmapped(value = 1) { value = 9; return arguments[0]; }
console.log("arguments", unmapped(3), unmapped());
function captured(value = 1, read = () => value) { value = 5; return read(); }
console.log("captured-parameter", captured());
