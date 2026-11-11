// M3: Padding truncation, empty fillers, repetition conversion, and errors.
console.log("padding", "7".padStart(4, "ab"), "7".padEnd(4, "ab"), "abc".padStart(2, "x"));
console.log("default-filler", "x".padStart(3) === "  x", "x".padEnd(3) === "x  ");
console.log("empty-filler", "x".padStart(5, ""), "x".padEnd(5, ""));
console.log("conversion", "x".padStart("3.9", 0), "ab".repeat("2.9"));
console.log("zero", "ab".repeat(0) === "", "ab".repeat(NaN) === "", "ab".repeat(-0) === "");
console.log("utf16", "x".padStart(2, "😀").charCodeAt(0), "😀".repeat(2).length);
for (const count of [-1, Infinity]) {
    try { "x".repeat(count); } catch (error) { console.log("repeat-error", error.name); }
}
