// M6: Var crosses blocks but not functions, and loop closures share one binding.
function outer() {
    if (true) { var value = 1; }
    for (var index = 0; index < 2; index++) { var last = index; }
    function inner() { var value = 9; return value; }
    return `${value}:${index}:${last}:${inner()}`;
}
console.log("scopes", outer(), typeof value, typeof index);
const shared = [];
for (var index = 0; index < 3; index++) { shared.push(() => index); }
const distinct = [];
for (let local = 0; local < 3; local++) { distinct.push(() => local); }
console.log("closures", shared[0](), shared[1](), shared[2](), distinct[0](), distinct[1](), distinct[2]());
function shadow() { var item = "outer"; { let item = "block"; console.log("block", item); } return item; }
console.log("lexical-shadow", shadow());
