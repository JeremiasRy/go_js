const numObjects = 1000000;
const objects = {};
const startTime = Date.now();

for (let i = 0; i < numObjects; i++) {
    const key = `obj-${i}`;
    objects[key] = {
        id: i,
        data: "some long string data",
        timestamp: Date.now()
    };
}

const endTime = Date.now();

console.log(`Created ${Object.keys(objects).length} objects.`);
console.log(`Execution Time: ${endTime - startTime}ms`)