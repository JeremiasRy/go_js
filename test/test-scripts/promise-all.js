// M4: All preserves input order, accepts iterables, and closes on resolve errors.
async function main() {
    let resolveFirst;
    let resolveSecond;
    const first = new Promise(resolve => { resolveFirst = resolve; });
    const second = new Promise(resolve => { resolveSecond = resolve; });
    const result = Promise.all([first, 3, second]);
    resolveSecond(2);
    resolveFirst(1);
    console.log("input-order", (await result).join(","));
    console.log("iterable", (await Promise.all(new Set([1, Promise.resolve(2)]))).join(","));
    let trace = "";
    const empty = Promise.all([]).then(values => { trace += "empty "; return values.length; });
    trace += "sync ";
    console.log("empty-before", trace);
    console.log("empty-after", await empty, trace);
    try { await Promise.all([Promise.resolve(1), Promise.reject("bad"), 3]); }
    catch (reason) { console.log("reject", reason); }
    let closed = 0;
    const iterable = { [Symbol.iterator]() { return { next() { return { value: 1, done: false }; }, return() { closed++; return {}; } }; } };
    function Constructor(executor) { return new Promise(executor); }
    Constructor.resolve = function () { throw new Error("resolve"); };
    try { await Promise.all.call(Constructor, iterable); }
    catch (error) { console.log("iterator-close", error.message, closed); }
    console.log("done");
}
main();
