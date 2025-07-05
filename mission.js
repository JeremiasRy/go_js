function main(fn) {
    const start = clock()
    const result = fn(5)
    console.log(clock() - start + "ms")
    return result
}

function fibo(n) {
    if (n <= 1) {
        return n
    }

    return fibo(n - 1) + fibo(n - 2)
}


console.log(main(fibo))