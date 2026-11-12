function descend(n) {
  return n === 0 ? 0 : descend(n - 1);
}
console.log(descend(63));
