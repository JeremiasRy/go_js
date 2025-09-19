function divide(a, b) {
    if (b === 0) {
        throw new Error("Division by zero is not allowed.");
    }
    return a / b;
}

try {
    divide(10, 0);
} catch (e) {
    console.log(e.message);
}