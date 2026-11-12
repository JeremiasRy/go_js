// M2: Prototype-chain tests, custom @@hasInstance, and invalid RHS values.
function Parent() {}
function Child() {}
Child.prototype = Object.create(Parent.prototype);
const child = new Child();
console.log("chain", child instanceof Child, child instanceof Parent, child instanceof Object, {} instanceof Parent);
const bound = Parent.bind(null);
console.log("bound", new Parent() instanceof bound, null instanceof Parent, 1 instanceof Parent);
const custom = { [Symbol.hasInstance](value) { return value === "accepted"; } };
console.log("custom", "accepted" instanceof custom, child instanceof custom);
for (const rhs of [null, 1, {}, { [Symbol.hasInstance]: 3 }]) {
    try { console.log(child instanceof rhs); } catch (error) { console.log("invalid-rhs", error.name); }
}
Parent.prototype = null;
try { console.log(child instanceof Parent); } catch (error) { console.log("invalid-prototype", error.name); }
