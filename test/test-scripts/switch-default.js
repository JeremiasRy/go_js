// M1: Evaluate the discriminant once, then select and fall through correctly.
let calls = 0;
let trace = "";
function discriminant(value) { calls++; return value; }
function match(value) { trace += "test" + value + " "; return value; }
function run(value) {
    switch (discriminant(value)) {
        case match(1): trace += "one "; break;
        default: trace += "default ";
        case match(2): trace += "two ";
        case match(3): trace += "three "; break;
    }
}
for (const value of [1, 2, 3, 9]) { trace = ""; run(value); console.log("switch", value, calls, trace); }
trace = "";
for (let index = 0; index < 3; index++) {
    switch (index) { case 0: continue; case 1: break; default: trace += "default "; }
    trace += index;
}
console.log("loop-targets", trace);
switch ("1") { case 1: console.log("wrong-coercion"); break; default: console.log("strict-match"); }
