console.log("1. Synchronous Code Start");

setTimeout(() => {
    console.log("5. Macrotask (setTimeout)");
}, 0);

Promise.resolve().then(() => {
    console.log("3. Microtask (Promise.then)");
    queueMicrotask(() => {
        console.log("4. Nested Microtask (queueMicrotask)");
    });
});


console.log("2. Synchronous Code End");
