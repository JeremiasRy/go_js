function Box(value) {
  this.value = value;
}

Box.prototype.get = function () {
  return this.value;
};

function make(n) {
  const box = new Box(n);
  return box.get();
}

function optional(fn, value) {
  return fn?.(value);
}

function spread(a, b, c) {
  return a + b + c;
}

function run(n) {
  let total = 0;
  for (let i = 0; i < n; i++) {
    total = total + make(i);
    total = total + optional((x) => x + 1, i);
    total = total + spread(...[i, i, i]);
    if ("value" in new Box(i)) {
      total = total + 1;
    }
    if (new Box(i) instanceof Box) {
      total = total + 1;
    }
  }
  return total;
}

for (let i = 0; i < 200; i++) {
  run(1);
}

console.log(run(10));
console.log(typeof run(1), optional(null, 1));
