const obj = {
  value: 7,
  method() {
    const arrow = () => this.value;
    return arrow();
  },
};

function readArrow(fn) {
  return fn();
}

for (let i = 0; i < 1000; i++) {
  readArrow(() => i);
  obj.method();
}

console.log(readArrow(() => 42));
console.log(obj.method());
