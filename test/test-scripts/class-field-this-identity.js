class A { x = this; y = this.x; constructor() { console.log("ctor", this === this.x, this === this.y); } }
const a = new A();
console.log("outside", a === a.x, a === a.y, a.x === a);
