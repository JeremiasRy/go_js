function sum(n) {
  let total = 0;
  for (let i = 0; i < n; i++) {
    total = total + i * 2;
  }
  return total;
}

console.log(sum(100000));
console.log(sum(10));
