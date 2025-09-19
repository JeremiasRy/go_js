const size = 500000;
const data = [];
for (let i = 0; i < size; i++) {
    data.push({ id: i, value: i * 2, name: `item-${i}` });
}

let sum = 0;
const startTime = Date.now();

for (const item of data) {
    sum += item.value;
}

const endTime = Date.now();

console.log(`Sum: ${sum}`);
console.log(`Execution Time: ${endTime - startTime}ms`);