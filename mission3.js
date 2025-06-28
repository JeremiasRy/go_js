function counter() {
    let count = 0

    return {
        increment: () => count++,
        count: () => count,
        superCount: function () {
            count = count + 2
            return count * 23
        }
    }
}
