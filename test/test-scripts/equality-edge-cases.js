// M1: Equality must distinguish numeric equality from truthiness and identity.
console.log("numbers", 1 == 2, -1 == 1, 0 === -0, NaN === NaN, NaN == NaN);
console.log("nullish", null == undefined, null === undefined, null == 0, undefined == 0);
console.log("coercion", " 1 " == 1, "" == 0, "0" == false, true == "1", "2" == true);
console.log("strict", 1 === "1", false === 0, "same" === "sa" + "me");
const first = {};
const alias = first;
console.log("identity", first === alias, first == {}, [] == [], [] == ![]);
console.log("array-coercion", [1] == 1, [] == "", [1, 2] == "1,2");
let calls = 0;
const numeric = { valueOf() { calls++; return 4; } };
console.log("object-primitive", numeric == 4, 4 == numeric, numeric === 4, calls);
console.log("same-object", numeric == numeric, calls);
console.log("nan-relations", NaN < 1, NaN >= 1, undefined == undefined);
