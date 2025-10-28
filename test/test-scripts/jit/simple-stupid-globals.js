let i = 0

function adder(n) {
    return n + i
}

while (i < 1001) {
    adder(i)
    i++
}