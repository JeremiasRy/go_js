// M4: Finally preserves outcomes unless cleanup throws or rejects.
async function main() {
    let trace = "";
    const fulfilled = Promise.resolve(7).finally(function () {
        trace += "finally:" + arguments.length + " ";
        return Promise.resolve().then(() => { trace += "cleanup "; return 99; });
    }).then(value => { trace += "then "; return value; });
    console.log("fulfilled", await fulfilled, trace);
    try { await Promise.reject("original").finally(() => "ignored"); }
    catch (reason) { console.log("preserved-rejection", reason); }
    try { await Promise.resolve(1).finally(() => { throw new Error("cleanup-throw"); }); }
    catch (error) { console.log("throw-overrides", error.message); }
    try { await Promise.reject("original").finally(() => Promise.reject("cleanup-reject")); }
    catch (reason) { console.log("reject-overrides", reason); }
    console.log("non-callable", await Promise.resolve(3).finally(1));
    console.log("done");
}
main();
