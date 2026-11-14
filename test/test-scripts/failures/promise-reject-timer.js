new Promise(function (resolve, reject) {
    setTimeout(function () {
        console.log("timer");
        reject(new Error("rejected"));
    }, 10);
}).catch(function (error) {
    console.log("caught", error.message);
});

setTimeout(function () {
    console.log("other timer");
}, 40);
