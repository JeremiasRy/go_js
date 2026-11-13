function churn(n) {
  let total = 0;
  let text = "";
  for (let i = 0; i < n; i++) {
    const item = { index: i, label: "item-" + i };
    total = total + item.index + item.label.length;
    text = item.label;
    const box = { nested: { value: item } };
    total = total + box.nested.value.index;
  }
  return total + text.length;
}

for (let i = 0; i < 200; i++) {
  churn(10);
}
console.log(churn(500));
