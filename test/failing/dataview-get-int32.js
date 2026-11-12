const b = new ArrayBuffer(4);
const v = new DataView(b);
v.setUint32(0, 0xffffffff);
console.log(v.getInt32(0));
