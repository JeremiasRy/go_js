let result1 = "hello" && "world";
console.log(result1);

let result2 = "" || "default";
console.log(result2);

let obj = null;
let n = obj && obj.name; // 'obj.name' is not evaluated because 'obj' is falsy.
console.log(n);