const numbers = [10, 20, 30];
const [a, b] = numbers;
console.log(`a: ${a}`)
console.log(`b: ${b}`)

const person = { name: "Charlie", age: 40 };
const { name, age: superAge } = person;
console.log(`name: ${name}`)
console.log(`age: ${superAge}`)