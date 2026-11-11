// M3: Noncoercing predicates, constants, radix conversion, and formatting.
for (const value of [NaN, Infinity, -Infinity, 0, -0, 1.5, "1", null, undefined]) {
    console.log("predicates", Number.isNaN(value), Number.isFinite(value), Number.isInteger(value), Number.isSafeInteger(value));
}
console.log("safe-boundary", Number.isSafeInteger(Number.MAX_SAFE_INTEGER), Number.isSafeInteger(Number.MAX_SAFE_INTEGER + 1));
console.log("constants", Number.MIN_SAFE_INTEGER === -Number.MAX_SAFE_INTEGER, Number.EPSILON > 0, 1 + Number.EPSILON > 1, Number.MIN_VALUE > 0, Number.MAX_VALUE < Infinity);
console.log("format", (1.25).toFixed(1), (123.456).toPrecision(4), (123).toExponential(2));
console.log("radix", (255).toString(16), (10).toString(2), (-10).toString(16));
console.log("special", NaN.toString(), Infinity.toFixed(2), (-0).toFixed(1));
for (const operation of [() => (1).toString(1), () => (1).toString(37), () => (1).toFixed(-1), () => (1).toPrecision(0)]) {
    try { operation(); } catch (error) { console.log("format-error", error.name); }
}
