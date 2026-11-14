async function main() {
    console.log("async start");
    const value = await new Promise(function (resolve) {
        setTimeout(function () {
            console.log("timer");
            resolve("resolved");
        }, 10);
    });
    console.log("async resumed", value);
}

main();

setTimeout(function () {
    console.log("other timer");
}, 40);

console.log("sync end");
