function counter() {
    let count = 0

    return {
        increment: () => count++,
        count: () => count,
    }
}


