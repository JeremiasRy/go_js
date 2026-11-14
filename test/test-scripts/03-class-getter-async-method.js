// Probe 03 — class getters and async methods are supported.
//
// Node 24 baseline: prints "A 1 1".
// go_js observed (2026-11-14): identical output. Classes, getters and async
// methods compile and run.

class A {
  constructor() {
    this.x = [];
  }
  get size() {
    return this.x.length;
  }
  push(v) {
    this.x.push(v);
  }
  async go() {
    return 1;
  }
}

const a = new A();
a.push(2);
console.log('A', a.size, await a.go());
