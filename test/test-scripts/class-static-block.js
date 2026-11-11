// M5: Static initialization order, scoped locals, this/super, and abrupt exit.
let trace = "";
class Parent { static label() { return "parent"; } }
class Example extends Parent {
    static first = (trace += "first ", 1);
    static { let local = 2; trace += "block-one "; this.second = this.first + local; }
    static third = (trace += "third ", 4);
    static { let local = 5; trace += "block-two "; this.fourth = local; this.parentLabel = super.label(); }
}
console.log("initialization", trace, Example.first, Example.second, Example.third, Example.fourth, Example.parentLabel, typeof local);
new Example();
new Example();
console.log("once", trace);
try {
    class Broken {
        static { trace += "throw "; throw new Error("static-block"); }
        static unreachable = (trace += "unreachable ", 1);
    }
    console.log(Broken);
} catch (error) { console.log("abrupt", error.message, trace); }
