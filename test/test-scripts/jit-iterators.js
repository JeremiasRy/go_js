function iterate(n) {
  const arr = [1, 2, 3];
  let total = 0;
  for (let round = 0; round < n; round++) {
    for (const value of arr) {
      total = total + value;
    }
    for (const key in arr) {
      total = total + Number(key);
    }
  }
  return total;
}

function restSum(...args) {
  let total = 0;
  for (const value of args) {
    total = total + value;
  }
  return total;
}

for (let i = 0; i < 5000; i++) {
  iterate(1);
  restSum(1, 2, 3);
}

console.log(iterate(10));
console.log(restSum(1, 2, 3, 4));
