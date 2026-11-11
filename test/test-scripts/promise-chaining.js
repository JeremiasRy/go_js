// M4: Then creates fresh promises and adopts handler return values/promises.
async function main() {
    const original = Promise.resolve(2);
    const first = original.then(value => value + 1);
    const second = first.then(value => Promise.resolve(value * 2));
    console.log("identity", first !== original, second !== first, Promise.resolve(original) === original);
    console.log("chain", await second, await original.then(), await original.then(null));
    const object = { value: 7 };
    console.log("pass-through", await Promise.resolve(object).then() === object);
    let resolveSelf;
    const self = new Promise(resolve => { resolveSelf = resolve; });
    resolveSelf(self);
    try { await self; } catch (error) { console.log("self-resolution", error.name); }
    let cycle;
    cycle = Promise.resolve().then(() => cycle);
    try { await cycle; } catch (error) { console.log("chain-cycle", error.name); }
    console.log("independent-branches", (await Promise.all([original.then(value => value + 10), original.then(value => value + 20)])).join(","));
    console.log("done");
}
main();
