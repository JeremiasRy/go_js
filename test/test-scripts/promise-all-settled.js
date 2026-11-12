// M4: AllSettled reports every outcome in input order.
async function main() {
    let resolveFirst;
    const first = new Promise(resolve => { resolveFirst = resolve; });
    const pending = Promise.allSettled([first, Promise.reject("bad"), 3]);
    resolveFirst("good");
    const results = await pending;
    for (const result of results) {
        console.log("result", result.status, result.value, result.reason, Object.hasOwn(result, "value"), Object.hasOwn(result, "reason"));
    }
    console.log("empty", (await Promise.allSettled([])).length);
    console.log("iterable", (await Promise.allSettled(new Set([1, 2]))).map(result => result.value).join(","));
    const broken = { [Symbol.iterator]() { return { next() { throw new Error("next"); } }; } };
    try { await Promise.allSettled(broken); } catch (error) { console.log("iterator-error", error.message); }
    console.log("done");
}
main();
