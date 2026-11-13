function descend(n) {
  return descend(n + 1);
}

try {
  descend(0);
} catch (e) {
  console.log(e.name);
}
console.log("after");
