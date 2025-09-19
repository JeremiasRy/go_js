try {
    const b = "jau"
    try {
        console.log(b)
        throw "hello"
    } catch {
        const innerCatchA = 12
        const innerCatchB = 12
        console.log(innerCatchA + innerCatchB)
    }
    throw new Error("jausers")
} catch (error) {
    console.log(error.message)
}
