// M1: Nullish coalescing is lazy and preserves falsy, non-nullish values.
let calls = 0;
function fallback() { calls++; return "fallback"; }
for (const value of [0, false, "", NaN, null, undefined]) {
    console.log("coalesce", value ?? fallback(), calls);
}
let reads = 0;
const object = { get value() { reads++; return 0; } };
console.log("getter", object.value ?? fallback(), reads, calls);
let assigned = 0;
console.log("assignment", 1 ?? (assigned = 9), assigned);
console.log("nested", null ?? undefined ?? "last");
console.log("precedence", (null ?? false) || "or", false || (0 ?? "unused"));
function fail() { throw new Error("must be skipped"); }
console.log("throw-skipped", "present" ?? fail());
