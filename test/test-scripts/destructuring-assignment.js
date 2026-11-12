// M2: Assignment patterns retain existing bindings and return the RHS.
let a = 1;
let b = 2;
[a, b] = [b, a];
console.log("swap", a, b);
const target = {};
const original = { value: 7, extra: 8 };
let rest;
console.log("rhs-identity", ({ value: target.answer, ...rest } = original) === original);
console.log("members", target.answer, rest.extra, "value" in rest);
let calls = 0;
function key() { calls++; return "value"; }
({ [key()]: a = 9, missing: b = a + 1 } = { value: undefined });
console.log("computed-defaults", a, b, calls);
const assigned = [3, 4];
console.log("array-rhs", ([a, b] = assigned) === assigned, a, b);
let writeOrder = "";
const receiver = { set first(value) { writeOrder += "first:" + value + " "; }, set second(value) { writeOrder += "second:" + value; } };
[receiver.first, receiver.second] = [5, 6];
console.log("write-order", writeOrder);
