// M2: Observable iterator acquisition and done/value access order.
let trace = "";
const iterable = {
    [Symbol.iterator]() {
        trace += "iterator ";
        let index = 0;
        const iterator = {
            get next() {
                trace += "next-method ";
                return function () {
                    console.log("next-receiver", this === iterator);
                    const current = index++;
                    return {
                        get done() { trace += "done" + current + " "; return current === 2; },
                        get value() { trace += "value" + current + " "; return current; }
                    };
                };
            }
        };
        return iterator;
    }
};
let values = "";
for (const value of iterable) { values += value; }
console.log("iteration", values, trace);
for (const invalid of [{}, { [Symbol.iterator]: 1 }, { [Symbol.iterator]() { return 1; } }, { [Symbol.iterator]() { return { next() { return 1; } }; } }]) {
    try { for (const value of invalid) { console.log("unexpected", value); } }
    catch (error) { console.log("invalid-iterator", error.name); }
}
