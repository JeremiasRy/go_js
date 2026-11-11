// M4: Rejection reasons, propagation, recovery, and handlers throwing.
async function main() {
    console.log("reject-recover", await Promise.reject("reason").catch(reason => "caught:" + reason));
    console.log("second-handler", await new Promise((resolve, reject) => reject(7)).then(undefined, reason => reason + 1));
    try { await Promise.reject("propagate").then(value => value); }
    catch (reason) { console.log("missing-reject-handler", reason); }
    try { await Promise.resolve(1).then(() => { throw new RangeError("handler"); }); }
    catch (error) { console.log("handler-throw", error.name, error.message); }
    try { await Promise.reject(undefined); }
    catch (reason) { console.log("undefined-reason", reason === undefined); }
    const recovered = Promise.reject("first")
        .catch(reason => Promise.reject(reason + ":second"))
        .catch(reason => reason + ":recovered");
    console.log("rejected-return", await recovered);
    console.log("done");
}
main();
