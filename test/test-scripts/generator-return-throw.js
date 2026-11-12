// M4: Injected completion, yielding finalizers, and reentrancy checks.
function* values() {
    try {
        try { yield "body"; } catch (error) { yield "caught:" + error; }
    } finally { yield "cleanup"; }
    return "normal";
}
const returned = values();
console.log("start", returned.next().value);
let result = returned.return("forced");
console.log("return-cleanup", result.value, result.done);
result = returned.next();
console.log("return-finished", result.value, result.done);
const thrown = values();
thrown.next();
console.log("caught", thrown.throw("injected").value, thrown.next().value);
result = thrown.next();
console.log("normal-return", result.value, result.done);
let started = false;
function* untouched() { started = true; yield 1; }
result = untouched().return(9);
console.log("return-before-start", started, result.value, result.done);
try { untouched().throw("before-start"); } catch (error) { console.log("throw-before-start", started, error); }
let active;
function* reentrant() {
    try { active.next(); } catch (error) { console.log("reentrant", error.name); }
    yield 1;
}
active = reentrant();
console.log("after-reentrancy", active.next().value);
