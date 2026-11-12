// M3: SameValue differs from strict equality for NaN and signed zero.
console.log("numbers", Object.is(NaN, NaN), Object.is(0, -0), Object.is(-0, -0), Object.is(1, 1), Object.is(1, 2));
console.log("types", Object.is(1, "1"), Object.is(null, undefined), Object.is(false, 0), Object.is(undefined, undefined));
const object = {};
console.log("identity", Object.is(object, object), Object.is(object, {}), Object.is([], []));
console.log("strings", Object.is("same", "sa" + "me"));
const symbol = Symbol("key");
console.log("symbols", Object.is(symbol, symbol), Object.is(symbol, Symbol("key")));
