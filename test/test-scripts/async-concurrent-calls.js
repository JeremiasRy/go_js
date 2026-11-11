// M4: Concurrent suspended invocations retain independent locals and handlers.
async function main() {
    let releaseFirst;
    let releaseSecond;
    const firstGate = new Promise(resolve => { releaseFirst = resolve; });
    const secondGate = new Promise(resolve => { releaseSecond = resolve; });
    let trace = "";
    async function worker(name, initial, gate) {
        let local = initial;
        const read = () => local;
        trace += "start:" + name + " ";
        try {
            local += await gate;
            await Promise.resolve();
            trace += "finish:" + name + " ";
            return read();
        } finally { trace += "finally:" + name + " "; }
    }
    const first = worker("first", 10, firstGate);
    const second = worker("second", 20, secondGate);
    console.log("started", trace);
    releaseSecond(2);
    console.log("second", await second);
    releaseFirst(1);
    console.log("first", await first);
    console.log("results", (await Promise.all([first, second])).join(","), trace);
    console.log("done");
}
main();
