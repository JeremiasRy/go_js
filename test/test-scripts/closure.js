function makeAdder(a, x, b) {
    console.log(`I'm just here to make this a bit harder ${a}, ${b}`)
    return function (y) {
        return x + y;
    };
}

const add5 = makeAdder(undefined, 5, "hello");
console.log(add5(2));
console.log(add5(10));

const add7 = makeAdder(undefined, 7, "hello");
console.log(add7(2));
console.log(add7(10));