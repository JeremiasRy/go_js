// M2: Copy own enumerable string/symbol properties from arbitrary expressions.
let trace = "";
const symbol = Symbol("visible");
const source = Object.create({ inherited: 1 });
Object.defineProperty(source, "hidden", { value: 2 });
Object.defineProperty(source, "visible", { enumerable: true, get() { trace += "get "; return 3; } });
source[symbol] = 4;
function make() { trace += "source "; return source; }
const copy = { start: 0, ...make(), visible: 5, ...{ end: 6 } };
console.log("copy", Object.keys(copy).join(","), copy.visible, copy[symbol], "hidden" in copy, "inherited" in copy, trace);
const descriptor = Object.getOwnPropertyDescriptor(copy, "visible");
console.log("data-property", descriptor.value, descriptor.writable, descriptor.get === undefined);
console.log("primitives", Object.keys({ ...null, ...undefined, ...false, ..."ab" }).join(","));
let gets = 0;
const { excluded, ...rest } = { get excluded() { gets++; return 7; }, kept: 8, [symbol]: 9 };
console.log("rest", excluded, gets, rest.kept, rest[symbol], Object.hasOwn(rest, "excluded"));
