// M5: Derived construction, super receivers, default constructors, and errors.
class Parent {
    constructor(name) { this.name = name; this.target = new.target; }
    describe() { return "parent:" + this.name; }
    static kind() { return "parent-kind"; }
}
class Child extends Parent {
    constructor(name) { super(name); this.extra = 1; }
    describe() { return super.describe() + ":child"; }
}
const child = new Child("Ada");
console.log("inheritance", child.describe(), child.extra, child instanceof Child, child instanceof Parent, child.target === Child);
console.log("chains", Object.getPrototypeOf(Child) === Parent, Object.getPrototypeOf(Child.prototype) === Parent.prototype, Child.kind());
console.log("super-receiver", Child.prototype.describe.call({ name: "other" }));
class DefaultChild extends Parent {}
console.log("default-constructor", new DefaultChild("Bob").describe());
class BeforeSuper extends Parent { constructor() { this.value = 1; super("late"); } }
try { new BeforeSuper(); } catch (error) { console.log("before-super", error.name); }
class DoubleSuper extends Parent { constructor() { super("first"); super("second"); } }
try { new DoubleSuper(); } catch (error) { console.log("double-super", error.name); }
class Replacement extends Parent { constructor() { return { replaced: true }; } }
console.log("replacement", new Replacement().replaced);
class PrimitiveReturn extends Parent { constructor() { super("x"); return 1; } }
try { new PrimitiveReturn(); } catch (error) { console.log("primitive-return", error.name); }
try { Parent("without-new"); } catch (error) { console.log("class-call", error.name); }
