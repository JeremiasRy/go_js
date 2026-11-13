class Counter {
  constructor(start) {
    this.value = start;
  }

  add(n) {
    this.value = this.value + n;
    return this.value;
  }

  get current() {
    return this.value;
  }
}

function run(n) {
  const counter = new Counter(0);
  let total = 0;
  for (let i = 0; i < n; i++) {
    total = total + counter.add(i);
  }
  return total + counter.current;
}

for (let i = 0; i < 5000; i++) {
  run(2);
}

console.log(run(100));
console.log(run(10));
