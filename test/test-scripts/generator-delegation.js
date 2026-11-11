// M4: yield* forwards inputs and abrupt completions to its delegate.
function* child() { const input = yield 1; return input + 2; }
function* parent() { const returned = yield* child(); yield "returned:" + returned; return 9; }
const generator = parent();
console.log("yield", generator.next().value, generator.next(5).value);
let result = generator.next();
console.log("complete", result.value, result.done);
let trace = "";
const iterable = {
    [Symbol.iterator]() {
        return {
            next(value) { trace += "next:" + value + " "; return { value: 3, done: false }; },
            return(value) { trace += "return:" + value + " "; return { value: "closed", done: true }; }
        };
    }
};
function* delegate() { return yield* iterable; }
const returning = delegate();
returning.next();
result = returning.return("forced");
console.log("forward-return", result.value, result.done, trace);
trace = "";
const throwing = delegate();
throwing.next();
try { throwing.throw("injected"); }
catch (error) { console.log("missing-throw", error.name, trace); }
function* recover() { try { yield 1; } catch (error) { yield "caught:" + error; } return 2; }
function* forwardThrow() { return yield* recover(); }
const recovered = forwardThrow();
recovered.next();
console.log("forward-throw", recovered.throw("value").value, recovered.next().value);
