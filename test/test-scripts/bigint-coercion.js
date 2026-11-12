// M6: Conversions, comparisons, fixed-width coercion, and JSON errors.
console.log("construct", BigInt("  -12 ").toString(), BigInt("0x10").toString(), BigInt("").toString(), BigInt(true).toString());
console.log("comparison", 1n == 1, 1n === 1, 1n == "1", 1n == "1.0", 9007199254740993n > 9007199254740992, 1n < Infinity, 1n < NaN);
console.log("string", String(12n), "value:" + 12n, typeof Object(1n), Object(1n).valueOf() === 1n);
console.log("width", BigInt.asIntN(8, 255n).toString(), BigInt.asUintN(8, -1n).toString(), BigInt.asIntN(0, 99n).toString());
console.log("number-rounding", BigInt(Number(9007199254740993n)) !== 9007199254740993n);
for (const operation of [() => new BigInt(1), () => BigInt(1.5), () => BigInt("1.5"), () => BigInt(null), () => BigInt.asIntN(-1, 1n), () => BigInt.asIntN(8, 1), () => JSON.stringify({ value: 1n })]) {
    try { operation(); console.log("missing-error"); }
    catch (error) { console.log("conversion-error", error.name); }
}
console.log("json-replacer", JSON.stringify({ value: 1n }, (key, value) => typeof value === "bigint" ? value.toString() : value));
