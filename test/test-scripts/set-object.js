const mySet = new Set();
const obj = { a: 1 };
mySet.add(1);
mySet.add("hello");
mySet.add(1);
mySet.add(obj);

console.log(mySet.has(1));
console.log(mySet.has(obj));
console.log(mySet.size);