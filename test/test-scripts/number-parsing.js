// M3: Prefix parsing, radices, whitespace, signed zero, and invalid inputs.
for (const text of ["  +12tail", "-12tail", "0x10", "010", "0b11", "", "word", "Infinity", "\u00A0\uFEFF42"]) {
    console.log("parse", text.length, parseInt(text), parseFloat(text));
}
console.log("radix", parseInt("101tail", 2), parseInt("ff", 16), parseInt("0x10", 0), parseInt("0x10", 16), parseInt("0x10", 10));
console.log("invalid-radix", Number.isNaN(parseInt("1", 1)), Number.isNaN(parseInt("1", 37)), Number.isNaN(parseInt("0x", 16)));
console.log("radix-conversion", parseInt("10", "2"), parseInt("10", 4294967298));
console.log("float", parseFloat("1.25e2rest"), parseFloat("1e+"), parseFloat(".5"), parseFloat("-Infinitytail"));
console.log("signed-zero", Object.is(parseInt("-0"), -0), Object.is(parseFloat("-0.0"), -0));
console.log("object-input", parseInt({ toString() { return "17tail"; } }), parseFloat({ toString() { return "2.5tail"; } }));
console.log("aliases", Number.parseInt === parseInt, Number.parseFloat === parseFloat);
