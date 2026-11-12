// M1: Comma expressions evaluate left-to-right and return the final value.
let trace = "";
function mark(value) { trace += value; return value; }
console.log("result", (mark("a"), mark("b"), mark("c")), trace);
let a = 0;
let b = 0;
console.log("assignment", (a = 1, b = a + 1, a + b), a, b);
let total = 0;
for (let left = 0, right = 3; left < right; left++, right--) { total += left + right; }
console.log("for-sequence", total);
const object = { method() { "use strict"; return this === undefined; } };
console.log("reference-discarded", object.method(), (0, object.method)());
