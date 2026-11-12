function f() {}
console.log(f.hasOwnProperty("prototype"));
const d = Object.getOwnPropertyDescriptor(f, "prototype");
console.log(d.value === f.prototype, d.writable, d.enumerable, d.configurable);
