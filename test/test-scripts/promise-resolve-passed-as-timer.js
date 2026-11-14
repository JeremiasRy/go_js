console.log("start");

new Promise(function (resolve) {
    setTimeout(resolve, 10);
}).then(function (value) {
    console.log("then", value);
});

setTimeout(function () {
    console.log("other timer");
}, 40);

console.log("sync end");
