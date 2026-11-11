// M6: Exact arithmetic, truncating division, signed shifts, and invalid operations.
console.log("arithmetic", (10n + 3n).toString(), (10n - 3n).toString(), (10n * 3n).toString(), (10n ** 20n).toString());
console.log("division", (7n / 3n).toString(), (-7n / 3n).toString(), (-7n % 3n).toString(), (7n % -3n).toString());
console.log("bitwise", (5n & 3n).toString(), (5n | 3n).toString(), (5n ^ 3n).toString(), (~0n).toString());
console.log("shifts", (1n << 65n).toString(), (-8n >> 2n).toString(), (8n << -1n).toString());
let value = 2n;
console.log("update", (value++).toString(), (++value).toString(), (value *= 3n).toString());
for (const operation of [() => 1n + 1, () => 1n * 2, () => +1n, () => 1n >>> 0n, () => 1n / 0n, () => 1n % 0n, () => 2n ** -1n]) {
    try { operation(); console.log("missing-error"); }
    catch (error) { console.log("invalid", error.name); }
}
