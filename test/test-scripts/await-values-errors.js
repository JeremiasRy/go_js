// M4: Await converts arbitrary values, and async calls always return promises.
async function main() {
    console.log("values", await 1, await null, await undefined, await "text");
    console.log("thenable", await { then(resolve) { resolve(7); } });
    async function throwsImmediately() { throw new TypeError("async-throw"); }
    const rejected = throwsImmediately();
    console.log("returned-promise", rejected instanceof Promise);
    try { await rejected; } catch (error) { console.log("caught", error.name, error.message); }
    let trace = "";
    try { trace += "try "; await Promise.reject("reason"); }
    catch (reason) { trace += "catch:" + reason + " "; }
    finally { await 0; trace += "finally "; }
    console.log("cleanup", trace);
    async function adopt(value) { return value; }
    const original = Promise.resolve(9);
    const adopted = adopt(original);
    console.log("adoption", adopted !== original, await adopted);
    async function override() { try { return 1; } finally { await 0; throw new Error("finalizer"); } }
    try { await override(); } catch (error) { console.log("override", error.message); }
    console.log("done");
}
main();
