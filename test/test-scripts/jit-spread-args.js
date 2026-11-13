function sum3(a, b, c) {
  return a + b + c;
}

function spreadCall(args) {
  return sum3(...args);
}

function restCall(...args) {
  return args[0] + args[1] + args[2];
}

for (let i = 0; i < 5000; i++) {
  spreadCall([i, i, i]);
  restCall(i, i, i);
}

console.log(spreadCall([1, 2, 3]));
console.log(restCall(4, 5, 6));
