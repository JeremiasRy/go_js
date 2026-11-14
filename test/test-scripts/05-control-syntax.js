// Probe 05 — control-flow / syntax constructs around the for(;;) crash.
//
// Node 24 baseline output:
//   bitshift: 0
//   for...of: 7,8
//   class loop: 2 ok
//   comparator: -1
//   getter chain: 0
//   undefined return: undefined
//
// go_js observed (2026-11-14): all of these passed. This narrows the compiler
// bug in probe 04 specifically to the empty-condition `for(;;)` head.

const a = [1, 2, 3];
console.log('bitshift:', (a.length >> 1) - 1);

const o = { items: [7, 8] };
const seen = [];
for (const x of o.items) seen.push(x);
console.log('for...of:', seen.join(','));

class H {
  constructor() {
    this.items = [1, 2];
  }
  get size() {
    return this.items.length;
  }
  go() {
    for (let i = (this.items.length >> 1) - 1; i >= 0; i--) {}
    return 'ok';
  }
}
const h = new H();
console.log('class loop:', h.size, h.go());

const cmp = (x, y) => (x.key - y.key) || (x.seq - y.seq);
console.log('comparator:', cmp({ key: 1, seq: 2 }, { key: 1, seq: 3 }));

class Inner {
  constructor() {
    this.items = [];
  }
  get size() {
    return this.items.length;
  }
}
class Outer {
  constructor() {
    this.heap = new Inner();
  }
  get size() {
    return this.heap.size;
  }
}
console.log('getter chain:', new Outer().size);

class Empty {
  constructor() {
    this.items = [];
  }
  pop() {
    const items = this.items;
    if (items.length === 0) return undefined;
  }
}
console.log('undefined return:', new Empty().pop());
