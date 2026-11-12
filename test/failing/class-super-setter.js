class A { get x() { return 1; } set x(v) { this.y = v; } }
class B extends A { get x() { return super.x + 1; } set x(v) { super.x = v + 1; } }
const b = new B();
console.log(b.x);
b.x = 4;
console.log(b.y);
