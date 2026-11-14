console.log("start");

new Promise(function (resolve) {
    console.log("executor");
    (function () {
        console.log("nested call");
        resolve("resolved");
        console.log("after resolve");
    })();
    console.log("executor end");
}).then(function (value) {
    console.log("then", value);
});

setTimeout(function () {
    console.log("other timer");
}, 40);

console.log("sync end");
