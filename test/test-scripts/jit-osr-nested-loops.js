function nested(n) {
  let total = 0;
  for (let i = 0; i < n; i++) {
    for (let j = 0; j < n; j++) {
      total = total + i * j;
    }
  }
  return total;
}

console.log(nested(200));
console.log(nested(3));
