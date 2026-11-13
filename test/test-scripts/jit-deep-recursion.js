function sumTo(n) {
  if (n === 0) return 0;
  return n + sumTo(n - 1);
}

try {
  sumTo(100000);
} catch (e) {
  console.log(e.name);
}
console.log(sumTo(100));
