// M5: Static fields/methods/accessors and computed names use the class receiver.
const key = "computed";
class Counter {
    static value = 1;
    static increment() { return ++this.value; }
    static get doubled() { return this.value * 2; }
    static set doubled(next) { this.value = next / 2; }
    static [key]() { return this.value; }
    get label() { return "instance"; }
}
console.log("static", Counter.increment(), Counter.doubled, Counter.computed());
Counter.doubled = 10;
console.log("setter", Counter.value, Counter.computed());
class Child extends Counter {}
console.log("inherited", Child.increment(), Child.value, Counter.value, Child.doubled);
const instance = new Counter();
console.log("placement", instance.value, instance.label, Counter.prototype.increment, Object.keys(Counter).join(","));
console.log("method-flags", Object.getOwnPropertyDescriptor(Counter, "increment").enumerable, Object.getOwnPropertyDescriptor(Counter.prototype, "label").enumerable);
