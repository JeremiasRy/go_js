// M6: Zeroed storage, views sharing memory, and copied slices.
const buffer = new ArrayBuffer(8);
const bytes = new Uint8Array(buffer);
console.log("zeroed", buffer.byteLength, bytes.join(","));
const view = new Uint8Array(buffer, 2, 3);
view.set([1, 2, 3]);
console.log("shared", view.buffer === buffer, view.byteOffset, view.byteLength, bytes.join(","));
const copy = buffer.slice(2, 5);
new Uint8Array(copy)[0] = 9;
console.log("copy", copy !== buffer, copy.byteLength, bytes[2], new Uint8Array(copy).join(","));
console.log("slice-bounds", buffer.slice(-2).byteLength, buffer.slice(6, 2).byteLength, buffer.slice(-99, 99).byteLength);
console.log("view-detection", ArrayBuffer.isView(bytes), ArrayBuffer.isView(new DataView(buffer)), ArrayBuffer.isView(buffer), ArrayBuffer.isView({}));
for (const operation of [() => new ArrayBuffer(-1), () => ArrayBuffer(1), () => new Uint8Array(buffer, 9), () => new Uint8Array(buffer, 7, 2)]) {
    try { operation(); } catch (error) { console.log("invalid-buffer", error.name); }
}
