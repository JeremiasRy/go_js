const message = "hello there!"
try {
    throw new Error(message)
} catch (error) {
    console.log(error.message)
    console.log(error[message])
}