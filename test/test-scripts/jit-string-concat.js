let plain = 1;
let text = "x";

function mixed(a, b, c) {
  return a + plain + text + b + c;
}

for (let i = 0; i < 10000; i++) {
  mixed(1, 2, 3);
}

console.log(mixed(1, 2, 3));
plain = "p";
text = 4;
console.log(mixed("a", "b", "c"));
