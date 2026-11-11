// M1: Finalizers run on normal and exceptional completion, with or without catch.
let trace = "";
try { trace += "try "; } finally { trace += "finally "; }
console.log("normal", trace);
trace = "";
try { trace += "try "; throw new Error("controlled"); }
catch (error) { trace += "catch:" + error.message + " "; }
finally { trace += "finally "; }
console.log("caught", trace);
trace = "";
try {
    try { throw "original"; } finally { trace += "inner "; }
} catch (error) { trace += error + " "; }
finally { trace += "outer"; }
console.log("propagation", trace);
function returned() { try { return 3; } finally { console.log("before-return"); } }
console.log("return", returned());
for (let index = 0; index < 3; index++) {
    try { if (index === 1) throw index; console.log("loop", index); }
    catch (error) { console.log("loop-catch", error); }
    finally { console.log("loop-finally", index); }
}
