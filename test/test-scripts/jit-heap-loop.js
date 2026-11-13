const state = { sum: 0, label: "start" };
const items = [1, 2, 3, 4];

function heapLoop(n) {
  let total = 0;
  for (let i = 0; i < n; i++) {
    state.sum = state.sum + i;
    items[i % items.length] = i;
    total = total + state.sum + items[0];
  }
  return total;
}

for (let i = 0; i < 5000; i++) {
  heapLoop(4);
}

console.log(heapLoop(50));
console.log(state.sum, state.label, items.length, items[1]);
