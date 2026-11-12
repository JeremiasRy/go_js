// M6: BigInt literals preserve arbitrary precision and radix/separator syntax.
for (const value of [0n, -0n, 123n, 9007199254740993n, 0xFFFFFFFFFFFFFFFFn, 0o777n, 0b1010n, 1_000_000n]) {
    console.log("literal", value.toString(), typeof value);
}
console.log("precision", (9007199254740993n + 1n).toString(), 9007199254740993n !== BigInt(9007199254740993));
console.log("radix", (255n).toString(16), (10n).toString(2), (-255n).toString(16));
console.log("separators", 0xFF_FFn === 65535n, 0b1010_0101n === 165n);
console.log("truthiness", !!0n, !!1n, !!-1n, 0n === -0n);
