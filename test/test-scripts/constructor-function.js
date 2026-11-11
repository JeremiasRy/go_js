// M2: Ordinary construction, returned objects, prototypes, and new.target.
function Item(value) { this.value = value; this.target = new.target; return 1; }
Item.prototype.read = function () { return this.value; };
const item = new Item(4);
console.log("constructed", item.read(), item.target === Item, Object.getPrototypeOf(item) === Item.prototype);
function Replacement() { return { replacement: true }; }
console.log("object-return", new Replacement().replacement);
function Target() { return new.target; }
console.log("ordinary-call", Target() === undefined, new Target() === Target);
function WithArrow() { this.target = (() => new.target)(); }
console.log("lexical-new-target", new WithArrow().target === WithArrow);
Item.prototype = { changed: true };
console.log("prototype-at-construction", item.changed, new Item(5).changed, item instanceof Item);
try { new ({ method() {} }).method(); } catch (error) { console.log("method-not-constructor", error.name); }
