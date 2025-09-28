const numIterations = 100000;
let longString = "";
const startTime = Date.now();

for (let i = 0; i < numIterations; i++) {
    longString += `This is a test string number ${i}. `;
}

const endTime = Date.now();

console.log(`Final string length: ${longString.length}`);
console.log(`Execution Time: ${endTime - startTime}ms`);