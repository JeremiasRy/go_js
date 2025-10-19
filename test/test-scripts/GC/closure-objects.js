function outer() {
    const largeObject = {
        data: new Array(1000).fill('some data')
    };

    return function inner() {
        return largeObject;
    };
}

for (let i = 0; i < 1000; i++) {
    let closure = outer();
    console.log(closure())
}
