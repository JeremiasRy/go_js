function outer() {
    const x = "value";
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