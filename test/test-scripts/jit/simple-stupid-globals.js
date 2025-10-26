let i = 0

function adder(n) {
    return n + i
}

while (i < 2) {
    adder(i)
    i++
}