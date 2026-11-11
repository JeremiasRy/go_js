// M4: Race settles on the first reaction; an empty race stays pending.
async function main() {
    console.log("first", await Promise.race([Promise.resolve("first"), "second"]));
    console.log("plain-value", await Promise.race(new Set([3, Promise.resolve(4)])));
    try { await Promise.race([Promise.reject("first-rejection"), 1]); }
    catch (reason) { console.log("rejection", reason); }
    let settled = false;
    Promise.race([]).then(() => { settled = true; }, () => { settled = true; });
    await Promise.resolve();
    await Promise.resolve();
    console.log("empty-pending", settled);
    let resolveLater;
    const later = new Promise(resolve => { resolveLater = resolve; });
    const race = Promise.race([later, 9]);
    console.log("plain-beats-pending", await race);
    resolveLater(99);
    const broken = { [Symbol.iterator]() { throw new Error("iterator"); } };
    try { await Promise.race(broken); } catch (error) { console.log("iterator-error", error.message); }
    console.log("done");
}
main();
