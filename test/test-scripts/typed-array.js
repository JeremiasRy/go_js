// M6: Numeric element coercion, views/copies, iteration, and BigInt storage.
console.log("uint8", new Uint8Array([-1, 256, 257, 1.9, NaN]).join(","));
console.log("int8", new Int8Array([127, 128, 255, 256]).join(","));
console.log("clamped", new Uint8ClampedArray([NaN, -1, 0.5, 1.5, 2.5, 3.5, 254.5, 255.5]).join(","));
const floats = new Float64Array([-0, NaN, Infinity, 1.5]);
console.log("float", Object.is(floats[0], -0), Number.isNaN(floats[1]), floats[2], floats[3]);
const values = new Uint8Array([1, 2, 3, 4]);
const shared = values.subarray(1, 3);
const copied = values.slice(1, 3);
shared[0] = 9;
copied[1] = 8;
console.log("views", values.join(","), shared.buffer === values.buffer, copied.buffer !== values.buffer);
values.set(values.subarray(0, 3), 1);
console.log("overlap", values.join(","));
console.log("factories", new Uint8Array({ 0: 5, length: 2 }).join(","), new Uint8Array(new Set([3, 1])).join(","));
console.log("methods", new Uint8Array([10, 2, 1]).sort().join(","), new Uint8Array([1, 2]).map(value => value * 2) instanceof Uint8Array, Int16Array.BYTES_PER_ELEMENT);
for (const operation of [() => new Int16Array(new ArrayBuffer(4), 1), () => values.set([1, 2], 3)]) {
    try { operation(); } catch (error) { console.log("bounds", error.name); }
}
const big = new BigInt64Array([-1n, 9223372036854775808n]);
console.log("bigint", big[0].toString(), big[1].toString(), new BigUint64Array(big.buffer)[0].toString());
try { big[0] = 1; } catch (error) { console.log("bigint-element", error.name); }
