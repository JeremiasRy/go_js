function fib(n) {
    if (n <= 1) {
        return n;
    }
    return fib(n - 1) + fib(n - 2);
}

const startTime = Date.now();
const result = fib(40);
const endTime = Date.now();

console.log(`Result: ${result}`);
console.log(`Execution Time: ${endTime - startTime}ms`);