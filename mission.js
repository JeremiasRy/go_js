function fibo(n) {
    if (n <= 1) {
        return n
    }

    return fibo(n - 1) + fibo(n - 2)
}

let start = clock()
fibo(35)
console.log(clock() - start + "ms")