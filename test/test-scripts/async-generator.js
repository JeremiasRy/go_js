// M4: Async generator requests queue and return promises with iterator results.
async function main() {
    async function* values() { const input = yield Promise.resolve(1); await 0; yield input; return 3; }
    const generator = values();
    console.log("self-iterable", generator[Symbol.asyncIterator]() === generator);
    const first = generator.next();
    const second = generator.next(7);
    const third = generator.next();
    console.log("promise-result", first instanceof Promise);
    for (const pending of [first, second, third]) {
        const result = await pending;
        console.log("queued", result.value, result.done);
    }
    async function* cleanup() {
        try { yield 1; } catch (reason) { yield "caught:" + reason; }
        finally { await 0; yield "cleanup"; }
    }
    const returned = cleanup();
    await returned.next();
    let result = await returned.return(9);
    console.log("return-cleanup", result.value, result.done);
    result = await returned.next();
    console.log("return-finished", result.value, result.done);
    const thrown = cleanup();
    await thrown.next();
    console.log("throw", (await thrown.throw("injected")).value, (await thrown.next()).value);
    console.log("done");
}
main();
