function count(...args) {
  return args.length;
}
const values = new Array(100000).fill(1);
console.log(count(...values));
