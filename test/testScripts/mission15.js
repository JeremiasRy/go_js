let num = 42;
const str = "Hello, JS!";
let bool = true;
let arr = [1, 2, 3];
let obj = { name: "Jeremias", age: 31 };
let undef;
let nul = null;

function greet(name) {
    return `Hello, ${name}!`;
}

const add = (a, b) => a + b;

if (num > 40) {
    console.log("Number is greater than 40");
} else {
    console.log("Number is 40 or less");
}

for (let i = 0; i < 3; i++) {
    console.log(`For loop iteration: ${i}`);
}

let count = 0;
while (count < 3) {
    console.log(`While loop iteration: ${count}`);
    count++;
}

arr.push(4);
arr.forEach(item => console.log(`Array item: ${item}`));
console.log("-- filtered --")
arr.filter(item => item > 2).forEach(item => console.log(`Filtered array item: ${item}`));

obj.job = "Trying my hardest to be a smart developer";
console.log(obj.name);

console.log(str.toUpperCase());
console.log(str.includes("JS"));

try {
    throw new Error("Test error");
} catch (error) {
    console.log(`Caught: ${error.message}`);
}

if (typeof document !== "undefined") {
    document.body.innerHTML = "<h1>Hello from JS!</h1>";
}

let message = "delayed message"

setTimeout(() => {
    console.log(`${message} ${count++}`);
    setTimeout(() => {
        console.log(`${message} ${count++}`);
    }, 4000)
}, 1000);

setTimeout(() => console.log(`${message} ${count++}`), 2000);
setTimeout(() => console.log(`${message} ${count++}`), 3000);
setTimeout(() => console.log(`${message} ${count++}`), 4000);