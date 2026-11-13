function Point(x, y) {
  this.x = x;
  this.y = y;
}

Point.prototype.sum = function () {
  return this.x + this.y;
};

function make(n) {
  const p = new Point(n, n + 1);
  return p.sum();
}

for (let i = 0; i < 5000; i++) {
  make(i);
}

console.log(make(1));
const p = new Point(3, 4);
console.log(p.x, p.y, p.sum());
