function outer() {
    const largeObject = {
        data: new Array(1000).fill('some data')
    };

    return function inner() {
        return largeObject;
    };
}

let closure = outer();

closure = null; 
