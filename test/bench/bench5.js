const numTrials = 10000;
let successCount = 0;
const startTime = Date.now();

for (let i = 0; i < numTrials; i++) {
    try {
        if (i % 2 === 0) {
            throw new Error("test");
        } else {
            successCount++;
        }
    } catch (e) {
        // catch the error
    }
}

const endTime = Date.now();

console.log(`Success count: ${successCount}`);
console.log(`Execution Time: ${endTime - startTime}ms`);