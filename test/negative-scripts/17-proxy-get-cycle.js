const p = new Proxy({}, {
  get() {
    return p.x;
  }
});
console.log(p.x);
