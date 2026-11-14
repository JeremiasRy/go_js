console.log("start");

(function () {
    function onMicrotask() {
        console.log("microtask");
    }

    queueMicrotask(onMicrotask);
})();

setTimeout(function () {
    console.log("other timer");
}, 40);

console.log("sync end");
