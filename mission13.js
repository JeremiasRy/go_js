
function err() {
    const a = "hello"
    try {
        const b = "jau"
        throw new Error("jausers")
    } catch (error) {
        console.log(a)
        console.log(b)
        console.log(error)
    }
}

err()