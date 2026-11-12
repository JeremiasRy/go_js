// M1: typeof, void, unary plus/minus, and bitwise NOT.
for (const value of [undefined, null, false, 1, "x", {}, [], function () {}]) {
    console.log("typeof", typeof value);
}
console.log("unresolved", typeof absentUnaryBinding);
let calls = 0;
console.log("void", void (calls++), calls, void 123 === undefined);
console.log("plus", +"12", +true, +false, +null, +" ");
console.log("nan", +undefined !== +undefined, +"x" !== +"x");
console.log("negative-zero", 1 / -0, 1 / +"-0", 1 / -(-0));
console.log("bitwise-not", ~0, ~1, ~-1, ~1.9, ~4294967296, ~NaN);
