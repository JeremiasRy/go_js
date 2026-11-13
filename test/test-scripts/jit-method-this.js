const counter = {
  value: 10,
  add(n) {
    return this.value + n;
  },
  twice() {
    return this.add(this.add(0));
  },
};

function callAdd(obj, n) {
  return obj.add(n);
}

for (let i = 0; i < 5000; i++) {
  callAdd(counter, i);
}

console.log(callAdd(counter, 5));
console.log(counter.twice());
