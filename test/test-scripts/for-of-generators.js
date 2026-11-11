// M4: Generators are iterable, expose return values only through next, and close.
let trace = "";
function* values() { try { yield 1; yield 2; return 99; } finally { trace += "cleanup "; } }
const generator = values();
console.log("self-iterable", generator[Symbol.iterator]() === generator);
console.log("spread", [...generator].join(","), trace);
trace = "";
for (const value of values()) { console.log("break-value", value); break; }
console.log("break-cleanup", trace);
trace = "";
const [first] = values();
console.log("pattern-cleanup", first, trace);
trace = "";
try { for (const value of values()) { throw new Error("body"); } }
catch (error) { console.log("throw-cleanup", error.message, trace); }
function* empty() { return 5; }
console.log("empty", [...empty()].length);
