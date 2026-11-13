function withDefault(a, b = 10, c = a + b) {
  return a + b + c;
}

for (let i = 0; i < 5000; i++) {
  withDefault(i, i, i);
}

console.log(withDefault(1));
console.log(withDefault(1, 2));
console.log(withDefault(1, 2, 3));
console.log(withDefault(1, undefined, 3));
