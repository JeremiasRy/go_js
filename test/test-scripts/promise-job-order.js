// M4: Promise reactions, nested jobs, and await continuations run FIFO.
async function main() {
    const trace = ["sync-start"];
    const first = Promise.resolve().then(() => {
        trace.push("first");
        Promise.resolve().then(() => trace.push("nested"));
    });
    const second = Promise.resolve().then(() => trace.push("second"));
    async function task() { trace.push("async-start"); await 0; trace.push("await"); }
    const third = task();
    trace.push("sync-end");
    console.log("synchronous", trace.join(","));
    await Promise.all([first, second, third]);
    console.log("jobs", trace.join(","));
    const settled = Promise.resolve();
    const one = settled.then(() => trace.push("late-one"));
    const two = settled.then(() => trace.push("late-two"));
    await Promise.all([one, two]);
    console.log("late-order", trace.slice(-2).join(","));
    console.log("done");
}
main();
