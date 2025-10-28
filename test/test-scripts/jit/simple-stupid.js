function adder(n) {
    if (n <= 1000) {
        return n + 99
    }
    return n + 2
}

let i = 0

while (i < 1002) {
    adder(i)
    i++
}