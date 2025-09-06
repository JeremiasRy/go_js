function outer() {
    const x = "value";
    const y = "neljätoista"
    console.log(y)
    function middle() {
        const midX = "middleX"
        const midXLocal = "midXLocal"
        console.log(midXLocal)
        function inner() {
            console.log(x)
            console.log(midX)
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