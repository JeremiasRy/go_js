function outer() {
    let a = 1;
    let b = 2;
    function middle() {
        let c = 3;
        let d = 4;
        function inner() {
            let e = 5
            console.log(a + b + c + d + e);
        }
        return inner
    }
    return middle
}

const mid = outer()
const inner = mid()
inner()
