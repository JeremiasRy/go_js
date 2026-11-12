const o = {};
Object.defineProperty(o, "x", {
  get() {
    return o.x;
  }
});
console.log(o.x);
