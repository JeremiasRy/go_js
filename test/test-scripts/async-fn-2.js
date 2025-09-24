
async function innerFunction() {
    console.log('2. Inside the inner function, before the await');
    await new Promise(resolve => setTimeout(() => resolve(), 100));
    console.log('4. Inside the inner function, after the await');
}

async function outerFunction() {
    console.log('1. Inside the outer function, before the await');
    await innerFunction();
    console.log('5. Inside the outer function, after the await');
}

console.log('0. Script start');
outerFunction();
console.log('3. Script end');
