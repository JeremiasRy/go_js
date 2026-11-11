// M1: A finalizer can replace a pending return/throw/break/continue.
function replaceReturn() { try { return 1; } finally { return 2; } }
function replaceThrow() { try { throw "original"; } finally { return "replacement"; } }
function throwOverReturn() { try { return 1; } finally { throw "finalizer"; } }
console.log("return", replaceReturn(), replaceThrow());
try { throwOverReturn(); } catch (error) { console.log("throw", error); }
let trace = "";
for (let index = 0; index < 3; index++) {
    try { if (index === 0) continue; break; }
    finally { trace += index; }
}
console.log("loop-cleanup", trace);
trace = "";
for (let index = 0; index < 3; index++) {
    try { break; } finally { trace += index; if (index < 2) continue; }
}
console.log("continue-over-break", trace);
function nested() {
    try { try { return "value"; } finally { trace += "inner "; } }
    finally { trace += "outer "; }
}
trace = "";
console.log("nested", nested(), trace);
