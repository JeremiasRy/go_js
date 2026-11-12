// M1: Every compound operator and observable reference evaluation order.
let value = 12;
console.log("arithmetic", value += 3, value -= 5, value *= 2, value /= 4, value %= 3, value **= 3);
console.log("bitwise", value <<= 2, value >>= 1, value >>>= 1, value &= 6, value |= 8, value ^= 3);
let text = "a";
console.log("concatenate", text += 2);
let trace = "";
let stored = 4;
const object = {
    get value() { trace += "get "; return stored; },
    set value(next) { trace += "set "; stored = next; }
};
function base() { trace += "base "; return object; }
function key() { trace += "key "; return "value"; }
function rhs() { trace += "rhs "; return 3; }
console.log("member", base()[key()] += rhs(), stored, trace);
let left;
let right;
console.log("chained", left = right = 7, left, right);
