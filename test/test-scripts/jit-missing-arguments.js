function add(a, b) {
  return a + b;
}

for (let i = 0; i < 10000; i++) {
  add(i, i);
}

console.log(add(1));
console.log(add(1, 2));
console.log(add(1, 2, 3));
