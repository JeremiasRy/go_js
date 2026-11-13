function double(x) {
  return x * 2;
}

function apply(fn, x) {
  return fn(x);
}

for (let i = 0; i < 10000; i++) {
  apply(double, i);
}

console.log(apply(double, 5));
console.log(double(21));
