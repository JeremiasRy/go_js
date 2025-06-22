function main(fn) {
    const start = clock()
    const result = fn(3)
    console.log(clock() - start + "ms")
    start = 123
    return result
}

function fibo(n) {
    if (n <= 1) {
        return n
    }

    return fibo(n - 1) + fibo(n - 2)
}

console.log(main(fibo))