console.log(5 & 1)
console.log(5 ^ 1)
console.log(5 | 1)
console.log(9.9 | 0);
console.log(-9.9 | 0);
console.log(12345.6789 & 12345.6789);
console.log(0.1 | 0);
console.log(-0.9 | 0);

const MAX_INT32 = 2147483647;

console.log(MAX_INT32 | 0);
console.log((MAX_INT32 + 1) | 0);
console.log((MAX_INT32 + 2) | 0);

const MIN_INT32 = -2147483648;
console.log(MIN_INT32 | 0);
console.log((MIN_INT32 - 1) | 0);

console.log(5 & 3);
console.log(5 | 3);
console.log(5 ^ 3);
