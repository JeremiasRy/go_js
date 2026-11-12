// M1/M2: Lazy assignment evaluates the reference and getter exactly once.
let trace = "";
let stored;
const object = {
    get value() { trace += "get "; return stored; },
    set value(next) { trace += "set "; stored = next; }
};
function base() { trace += "base "; return object; }
function key() { trace += "key "; return "value"; }
function rhs() { trace += "rhs "; return 9; }
for (const initial of [0, false, "", null, undefined, 3]) {
    stored = initial; trace = "";
    console.log("and", base()[key()] &&= rhs(), trace);
    stored = initial; trace = "";
    console.log("or", base()[key()] ||= rhs(), trace);
    stored = initial; trace = "";
    console.log("nullish", base()[key()] ??= rhs(), trace);
}
const truthy = 1;
const falsy = 0;
console.log("no-write-to-const", truthy ||= 2, falsy &&= 2, truthy ??= 2);
