console.log("start");

(function () {
    function onTimeout() {
        console.log("timeout");
    }

    setTimeout(onTimeout, 10);
})();

setTimeout(function () {
    console.log("other timer");
}, 40);

console.log("sync end");
