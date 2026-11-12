// M6: Var declarations initialize to undefined and allow redeclaration.
console.log("top-level-before", hoisted);
var hoisted = 1;
var hoisted;
var hoisted = 2;
console.log("top-level-after", hoisted);
function scope() {
    console.log("function-before", local);
    if (false) { var local = 9; }
    console.log("unexecuted-declaration", local);
    local = 3;
    var local;
    return local;
}
console.log("function-after", scope(), typeof local);
function functionBinding() {
    console.log("function-initialized", typeof value, value());
    var value;
    function value() { return 7; }
    return typeof value;
}
console.log("var-does-not-overwrite-function", functionBinding());
function parameter(value) { var value; return value; }
console.log("parameter-redeclaration", parameter(8));
