// M5: Fields initialize per instance, in order, after super in derived classes.
let trace = "";
let keys = 0;
function initialize(label, value) { trace += label + " "; return value; }
class Parent {
    parent = initialize("parent-field", 1);
    constructor() { trace += "parent-body "; }
    set value(next) { trace += "inherited-setter "; }
}
class Child extends Parent {
    object = initialize("object-field", {});
    value = initialize("value-field", this.parent + 1);
    [(() => { keys++; return "computed"; })()] = 3;
    empty;
    constructor() { super(); trace += "child-body "; }
}
console.log("key-at-definition", keys, trace);
const first = new Child();
console.log("first", trace, first.value, first.computed, Object.hasOwn(first, "empty"), first.empty);
trace = "";
const second = new Child();
first.object.value = 9;
console.log("second", trace, keys, first.object !== second.object, second.object.value);
console.log("data-property", Object.getOwnPropertyDescriptor(first, "value").value);
