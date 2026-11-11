// M1: ToInt32/ToUint32 and shift-count masking.
for (const count of [0, 1, 31, 32, 33, -1]) {
    console.log("shift", count, 5 << count, -8 >> count, -8 >>> count);
}
console.log("conversion", "8" >> "1", 9.9 >> 1, -9.9 >> 1, NaN << 2, Infinity >>> 0);
console.log("wrap", 2147483648 >> 0, 4294967297 << 0, -1 >>> 0);
console.log("unsigned", -2147483648 >>> 31, 0x80000000 >> 31);
