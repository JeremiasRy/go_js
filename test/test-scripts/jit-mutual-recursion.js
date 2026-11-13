function isEven(n) {
  if (n === 0) return true;
  return isOdd(n - 1);
}

function isOdd(n) {
  if (n === 0) return false;
  return isEven(n - 1);
}

for (let i = 0; i < 5000; i++) {
  isEven(10);
  isOdd(10);
}

console.log(isEven(100));
console.log(isOdd(101));
console.log(isEven(101));
console.log(isOdd(100));
