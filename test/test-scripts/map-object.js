const myMap = new Map();
myMap.set("a", 1);
myMap.set("b", 2);

console.log(myMap.get("a"));
console.log(myMap.has("b"));
console.log(myMap.size);

for (const key of myMap.keys()) {
    console.log(key)
}
