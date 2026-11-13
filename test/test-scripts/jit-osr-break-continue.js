function mixed(n) {
  let total = 0;
  let seen = 0;
  for (let i = 0; i < n; i++) {
    if (i % 7 === 0) continue;
    if (i > n - 30) break;
    total = total + i;
    seen = seen + 1;
  }
  return total * 1000 + seen;
}

console.log(mixed(20000));
console.log(mixed(10));
