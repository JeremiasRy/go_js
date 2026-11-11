// M4: Assimilation retrieves then once and invokes it in a later job.
async function main() {
    let trace = "";
    let reads = 0;
    const thenable = {
        get then() {
            reads++;
            trace += "get ";
            return function (resolve, reject) {
                trace += "call:" + (this === thenable) + " ";
                resolve({ then(resolve) { trace += "nested "; resolve(9); } });
                reject("ignored");
                resolve(99);
                throw new Error("ignored after resolve");
            };
        }
    };
    const promise = Promise.resolve(thenable);
    trace += "sync ";
    console.log("before-job", trace, reads);
    console.log("assimilated", await promise, trace, reads);
    const badGetter = { get then() { throw new SyntaxError("then-getter"); } };
    try { await Promise.resolve(badGetter); }
    catch (error) { console.log("getter-error", error.name, error.message); }
    const plain = { then: 3 };
    console.log("non-callable-then", await Promise.resolve(plain) === plain);
    try { await Promise.resolve({ then(resolve, reject) { reject("first"); resolve("second"); } }); }
    catch (reason) { console.log("first-rejection", reason); }
    console.log("done");
}
main();
