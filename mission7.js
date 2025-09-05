function outer() {
    const jau = "jau"
    const number = 123
    const x = "value";
    const y = "neljätoista"
    console.log(y)
    console.log(number)
    console.log(jau)
    function middle() {
        function inner() {
            console.log(x)
        }

        console.log("create inner closure");
        return inner;
    }

    console.log("return from outer");
    return middle;
}

const mid = outer();
const inner = mid();
inner();