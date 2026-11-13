function double(x) {
  return x * 2;
}

let total = 0;
for (let i = 0; i < 5000; i++) {
  total = total + double(i);
}
console.log(total);

const arr = [1, 2, 3];
let mapped = 0;
for (let i = 0; i < 5000; i++) {
  mapped = mapped + arr.map(double)[2];
}
console.log(mapped);
