// M2: Array/call spread and patterns share the iterator protocol.
let calls = 0;
const iterable = {
    [Symbol.iterator]() {
        calls++;
        let next = 1;
        return { next() { return { value: next++, done: next > 4 }; } };
    }
};
console.log("array", [0, ...iterable, 4].join(","), calls);
function sum(a, b, c) { return a + b + c; }
console.log("call", sum(...iterable), calls);
const [first, , third] = iterable;
console.log("pattern", first, third, calls);
console.log("code-points", [..."A😀B"].join("|"), [..."A😀B"].length);
const sparse = [, 1];
const spread = [...sparse];
console.log("holes-materialized", 0 in sparse, 0 in spread, spread[0]);
let trace = "";
function mark(value) { trace += value; return value; }
console.log("argument-order", sum(mark(1), ...[mark(2)], mark(3)), trace);
try { console.log([...{ 0: 1, length: 1 }]); } catch (error) { console.log("array-like-not-iterable", error.name); }
