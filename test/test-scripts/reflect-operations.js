// M6: Reflect exposes the shared property, prototype, call, and construct rules.
const parent = { inherited: 1, get value() { return this.own; }, set value(next) { this.own = next; } };
const target = Object.create(parent);
const receiver = { own: 2 };
console.log("get", Reflect.get(target, "value", receiver), Reflect.has(target, "inherited"));
console.log("set", Reflect.set(target, "value", 3, receiver), receiver.own, target.own);
console.log("define", Reflect.defineProperty(target, "fixed", { value: 4, enumerable: true }), Reflect.getOwnPropertyDescriptor(target, "fixed").value);
console.log("failed-operations", Reflect.set(target, "fixed", 5), Reflect.deleteProperty(target, "fixed"));
const symbol = Symbol("key");
target[symbol] = 6;
console.log("own-keys", Reflect.ownKeys(target).map(key => typeof key === "symbol" ? "symbol" : key).join(","));
console.log("prototype", Reflect.getPrototypeOf(target) === parent, Reflect.setPrototypeOf(target, null));
console.log("extensibility", Reflect.isExtensible(target), Reflect.preventExtensions(target), Reflect.isExtensible(target), Reflect.defineProperty(target, "new", { value: 1 }));
function sum(a, b) { return this.offset + a + b; }
console.log("apply", Reflect.apply(sum, { offset: 10 }, [1, 2]));
function Item(value) { this.value = value; this.target = new.target; }
function Alternate() {}
const item = Reflect.construct(Item, [7], Alternate);
console.log("construct", item.value, item.target === Alternate, Object.getPrototypeOf(item) === Alternate.prototype);
for (const operation of [() => Reflect.get(1, "x"), () => Reflect.apply({}, null, []), () => Reflect.construct(() => 1, [])]) {
    try { operation(); } catch (error) { console.log("invalid-target", error.name); }
}
