try {
    throw new Error("test error")
} catch (error) {
    console.log(`Caught: ${error.message}`)
}