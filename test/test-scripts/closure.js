function makeAdder(x) {
    return function (y) {
        return x + y;
    };
}

const add5 = makeAdder(5);
console.log(add5(2));
console.log(add5(10));

const add7 = makeAdder(7);
console.log(add7(2));
console.log(add7(10));