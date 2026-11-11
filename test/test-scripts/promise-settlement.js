// M4: Synchronous executors, settlement-once, and multiple reaction listeners.
async function main() {
    let trace = "";
    let resolvePending;
    let rejectPending;
    const pending = new Promise((resolve, reject) => {
        trace += "executor ";
        resolvePending = resolve;
        rejectPending = reject;
    });
    const first = pending.then(value => { trace += "first:" + value + " "; return value + 1; });
    const second = pending.then(value => { trace += "second:" + value + " "; return value + 2; });
    resolvePending(3);
    resolvePending(99);
    rejectPending("ignored");
    trace += "sync ";
    console.log("before-jobs", trace);
    console.log("listeners", (await Promise.all([first, second])).join(","), trace);
    console.log("late-listener", await pending.then(value => value * 2));
    console.log("throw-after-resolve", await new Promise(resolve => { resolve(7); throw new Error("ignored"); }));
    try { await new Promise(() => { throw new TypeError("executor"); }); }
    catch (error) { console.log("executor-error", error.name, error.message); }
    try { new Promise(1); } catch (error) { console.log("invalid-executor", error.name); }
    console.log("done");
}
main();
