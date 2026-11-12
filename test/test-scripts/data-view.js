// M6: Explicit endianness, signed conversions, floating values, and bounds.
const buffer = new ArrayBuffer(24);
const view = new DataView(buffer, 2, 20);
view.setUint16(0, 0x1234);
console.log("big-endian", view.getUint8(0), view.getUint8(1), view.getUint16(0), view.getUint16(0, true));
view.setUint32(2, 0x01020304, true);
console.log("little-endian", view.getUint8(2), view.getUint8(5), view.getUint32(2, true), view.getUint32(2));
view.setInt8(6, -1);
view.setInt16(7, 65535, true);
console.log("signed", view.getInt8(6), view.getUint8(6), view.getInt16(7, true));
view.setFloat32(0, 1.5, true);
view.setFloat64(8, -0, false);
console.log("floats", view.getFloat32(0, true), Object.is(view.getFloat64(8), -0));
view.setBigInt64(0, -1n, true);
console.log("bigint", view.getBigInt64(0, true).toString(), view.getBigUint64(0, true).toString());
console.log("view", view.buffer === buffer, view.byteOffset, view.byteLength);
let conversions = 0;
view.setUint8(0, { valueOf() { conversions++; return 257; } });
console.log("conversion", view.getUint8(0), conversions);
for (const operation of [() => view.getUint32(18), () => view.setUint8(-1, 0), () => new DataView(buffer, 25)]) {
    try { operation(); } catch (error) { console.log("bounds", error.name); }
}
