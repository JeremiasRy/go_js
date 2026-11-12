// M3: Primitive constructor calls, wrapper objects, and conversion hooks.
console.log("defaults", Number(), String() === "", Boolean());
console.log("number", Number(" 12 "), Number(null), Number(true), Number("0x10"), Number("0b11"), Number.isNaN(Number(undefined)));
console.log("string", String(null), String(undefined), String(false), String(-0));
console.log("boolean", Boolean(-1), Boolean(""), Boolean("false"), Boolean([]), Boolean(NaN));
const number = new Number(7);
const string = new String("abc");
const boolean = new Boolean(false);
console.log("wrappers", typeof number, number.valueOf(), typeof string, string.length, string[1], boolean.valueOf(), !!boolean);
console.log("wrapper-equality", number == 7, number === 7, string == "abc");
const object = { valueOf() { return 9; }, toString() { return "text"; } };
console.log("hooks", Number(object), String(object));
console.log("constants", NaN !== NaN, Infinity > 1e300, -Infinity < -1e300);
