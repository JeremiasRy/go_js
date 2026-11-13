let empty = 0;
for (let i = 0; i < 5000; i++) {
}
console.log(empty);

let caught = 0;
for (let i = 0; i < 5000; i++) {
  try {
    if (i % 2 === 0) {
      throw new Error("even");
    }
    empty = empty + 1;
  } catch (e) {
    caught = caught + 1;
  }
}
console.log(empty, caught);
