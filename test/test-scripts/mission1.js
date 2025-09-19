
let i = 0
function looper1() {
    while (++i < 5) {
        console.log(i)
    }
}

function looper2() {
    while (i-- > 0) {
        console.log(i)
    }
}

looper1()
looper2()
