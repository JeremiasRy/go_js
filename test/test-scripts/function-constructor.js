console.log(typeof Function, Function.length);
const f = new Function("a", "return a + 1");
console.log(f(2));
