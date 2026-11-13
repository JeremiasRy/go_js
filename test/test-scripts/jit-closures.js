function makeAdder(offset) {
  return function add(x) {
    return x + offset;
  };
}

function apply(fn, value) {
  return fn(value);
}

const add10 = makeAdder(10);
const add20 = makeAdder(20);

for (let i = 0; i < 5000; i++) {
  apply(add10, i);
  apply(add20, i);
}

console.log(apply(add10, 1));
console.log(apply(add20, 2));
console.log(add10(add20(0)));
