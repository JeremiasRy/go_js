// M1: Logical expressions return operands and only evaluate needed branches.
let calls = 0;
function effect(value) { calls++; return value; }
function fail() { throw new Error("must be skipped"); }
console.log("skip", false && effect(1), true || effect(2), calls);
console.log("take", true && effect("right"), false || effect("fallback"), calls);
console.log("operands", "left" || fail(), 0 && fail(), "" && fail(), -2 && "yes");
let assigned = 0;
false && (assigned = 1);
true || (assigned = 2);
console.log("assignment-skipped", assigned);
console.log("left-once", effect("left") || effect("right"), calls);
console.log("nested", (false || effect(4)) && effect(5), calls);
