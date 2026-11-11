// M4: Any ignores rejections until every input rejects; errors retain input order.
async function main() {
    console.log("winner", await Promise.any([Promise.reject("bad"), Promise.resolve("good"), "later"]));
    console.log("iterable", await Promise.any(new Set([5, 6])));
    let rejectFirst;
    let rejectSecond;
    const first = new Promise((resolve, reject) => { rejectFirst = reject; });
    const second = new Promise((resolve, reject) => { rejectSecond = reject; });
    const result = Promise.any([first, second]);
    rejectSecond("second");
    rejectFirst("first");
    try { await result; }
    catch (error) { console.log("aggregate", error.name, error instanceof AggregateError, error instanceof Error, error.errors.join(",")); }
    try { await Promise.any([]); }
    catch (error) { console.log("empty", error.name, error.errors.length); }
    try { await Promise.any({ [Symbol.iterator]() { throw new Error("iterator"); } }); }
    catch (error) { console.log("iterator-error", error.name, error.message); }
    console.log("done");
}
main();
