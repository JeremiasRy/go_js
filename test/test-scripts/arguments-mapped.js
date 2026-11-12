function f(a) { arguments[0] = 9; return a; }
console.log(f(1));
function g(a) { a = 7; return arguments[0]; }
console.log(g(1));
function h(a, b = 2) { arguments[0] = 9; return a; }
console.log(h(1));
