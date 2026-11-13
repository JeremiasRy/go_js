function descend(n) {
  if (n < 0) {
    return descend(n - 1);
  }
  return 0;
}

try {
  descend(-1);
} catch (e) {
  console.log(e.name);
}
console.log(descend(1));
