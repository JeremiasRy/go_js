function loop() {
  let last = 0;
  let count = 0;
  for (let i = 0; i < 5000; i++) {
    last = i;
    count = count + 1;
  }
  return last * 100000 + count;
}

console.log(loop());

let out = "";
for (let i = 0; i < 5000; i++) {
  out = "x";
}
console.log(out);
