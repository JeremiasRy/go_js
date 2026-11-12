// M2: Elisions consume iterator steps; rest exhausts into a fresh array.
let trace = "";
const iterable = {
    [Symbol.iterator]() {
        let index = 0;
        return {
            next() { trace += "next "; return { value: index++, done: index > 4 }; },
            return() { trace += "close "; return { done: true }; }
        };
    }
};
const [, second, ...tail] = iterable;
console.log("rest", second, tail.join(","), trace);
trace = "";
const [first] = iterable;
console.log("early-close", first, trace);
const original = [1, 2];
const [...copy] = original;
copy[0] = 9;
console.log("fresh", copy !== original, original[0], copy[0]);
const [missing = 7, ...empty] = [];
console.log("empty", missing, empty.length);
const [letter, ...letters] = "A😀B";
console.log("string", letter, letters.join("|"));
