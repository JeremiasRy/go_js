function addOne(x) {
  return x + 1;
}

for (let i = 0; i < 10000; i++) {
  addOne(i);
}

console.log(addOne("abc"));
console.log(addOne(true));
console.log(addOne(null));
