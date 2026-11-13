function answer() {
  let value = 42;
  return value;
}

for (let i = 0; i < 10000; i++) {
  answer();
}

console.log(answer() + 1);
console.log(answer() * 2);
