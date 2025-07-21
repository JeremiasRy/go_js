function outer() {
    let a = 1;
    let b = 2;
    function middle() {
        let c = 3;
        let d = 4;
        function inner() {
            console.log(a + b + c + d);
        }
        return inner
    }
    return middle
}

const mid = outer()
const inner = mid()
inner()
