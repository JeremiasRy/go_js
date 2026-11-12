// M1/M2: Prefix/postfix updates coerce, write back, and evaluate once.
let trace = "";
let stored = "4";
const object = {
    get value() { trace += "get "; return stored; },
    set value(next) { trace += "set "; stored = next; }
};
function base() { trace += "base "; return object; }
function key() { trace += "key "; return "value"; }
console.log("postfix", base()[key()]++, stored, trace);
trace = "";
console.log("prefix", ++base()[key()], stored, trace);
trace = "";
console.log("decrement", base()[key()]--, --base()[key()], stored, trace);
const array = [10];
let index = 0;
console.log("index-once", array[index++]++, index, array[0]);
let value = 1;
console.log("local", value++, ++value, value--, --value, value);
