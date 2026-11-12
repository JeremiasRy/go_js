// M4: Async iteration, sync fallback, awaited cleanup, and rejection propagation.
async function main() {
    let trace = "";
    for await (const value of [Promise.resolve(1), 2]) { trace += value; }
    console.log("sync-fallback", trace);
    trace = "";
    const iterable = {
        [Symbol.asyncIterator]() {
            let index = 0;
            return {
                next() { return Promise.resolve({ value: index++, done: index > 3 }); },
                return() { trace += "close-start "; return Promise.resolve().then(() => { trace += "close-end "; return {}; }); }
            };
        }
    };
    for await (const value of iterable) { trace += "body:" + value + " "; break; }
    trace += "after ";
    console.log("awaited-close", trace);
    trace = "";
    try { for await (const value of iterable) { trace += "body "; throw new Error("body"); } }
    catch (error) { console.log("body-error-cleanup", error.message, trace); }
    const badClose = {
        [Symbol.asyncIterator]() {
            return {
                next() { return Promise.resolve({ value: 1, done: false }); },
                return() { return Promise.reject(new Error("close")); }
            };
        }
    };
    try { for await (const value of badClose) { break; } }
    catch (error) { console.log("close-rejection", error.message); }
    try { for await (const value of badClose) { throw new Error("body-wins"); } }
    catch (error) { console.log("completion-precedence", error.message); }
    async function* values() { yield Promise.resolve(3); yield 4; }
    trace = "";
    for await (const value of values()) { trace += value; }
    console.log("async-generator", trace);
    const broken = { [Symbol.asyncIterator]() { return { next() { return Promise.reject(new Error("next")); } }; } };
    try { for await (const value of broken) {} }
    catch (error) { console.log("next-rejection", error.message); }
    try { for await (const value of [Promise.reject("value")]) {} }
    catch (reason) { console.log("value-rejection", reason); }
    console.log("done");
}
main();
