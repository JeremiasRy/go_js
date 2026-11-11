// M1: ToBoolean, primitive conversions, and ordinary ToPrimitive.
for (const value of [-1, -Infinity, 1, Infinity, 0, -0, NaN, "", "0", null, undefined, false, true, [], {}]) {
    console.log("truthy", !!value);
}
console.log("numbers", +"  12  ", +"", +"\t\n", +null, +true, +false);
console.log("invalid", +undefined !== +undefined, +"no" !== +"no");
console.log("operators", "7" - 2, "6" * "3", true + 2, null + 1, "1" + 2);
console.log("strings", "" + null, "" + undefined, "" + false, "" + [1, 2]);
let trace = "";
const object = {
    valueOf() { trace += "valueOf "; return 7; },
    toString() { trace += "toString "; return "nine"; }
};
console.log("primitive", object + 1, `${object}`, trace);
const fallback = { valueOf() { return {}; }, toString() { return "12"; } };
console.log("fallback", fallback - 2);
try {
    console.log(+{ valueOf() { return {}; }, toString() { return {}; } });
} catch (error) {
    console.log("invalid-primitive", error.name);
}
