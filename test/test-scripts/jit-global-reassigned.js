let offset = 1;

function addOffset(x) {
  return x + offset;
}

for (let i = 0; i < 10000; i++) {
  addOffset(i);
}

offset = "z";
console.log(addOffset(1));
