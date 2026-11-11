// M1: Function declarations are initialized before their bodies execute.
console.log("top-level", add(2, 3));
function add(a, b) { return a + b; }
function outer() {
    console.log("nested", inner());
    function inner() { return "inner"; }
    return inner;
}
console.log("returned", outer()());
function scoped() {
    "use strict";
    {
        console.log("block", local());
        function local() { return "block-local"; }
    }
    console.log("outside-block", typeof local);
}
scoped();
try { expression(); const expression = () => 1; }
catch (error) { console.log("expression-tdz", error.name); }
