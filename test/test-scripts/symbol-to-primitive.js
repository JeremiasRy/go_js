// M2: @@toPrimitive receives a hint and supersedes ordinary conversion hooks.
let trace = "";
const object = {
    [Symbol.toPrimitive](hint) { trace += hint + " "; return hint === "string" ? "text" : 7; },
    valueOf() { throw new Error("valueOf should be skipped"); },
    toString() { throw new Error("toString should be skipped"); }
};
console.log("conversion", +object, String(object), object + 1, object == 7, trace);
const keyObject = { [Symbol.toPrimitive](hint) { console.log("key-hint", hint); return "key"; } };
console.log("property-key", ({ [keyObject]: 9 }).key);
for (const invalid of [{ [Symbol.toPrimitive]() { return {}; } }, { [Symbol.toPrimitive]: 1 }]) {
    try { console.log(+invalid); } catch (error) { console.log("invalid", error.name); }
}
const fallback = { [Symbol.toPrimitive]: null, valueOf() { return 4; } };
console.log("null-hook", +fallback);
