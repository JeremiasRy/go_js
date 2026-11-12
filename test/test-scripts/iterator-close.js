// M2: Closing and completion precedence on early loop/destructuring exits.
let trace = "";
function source(closeThrows = false) {
    return {
        [Symbol.iterator]() {
            let index = 0;
            return {
                next() { trace += "next "; return { value: index++, done: index > 2 }; },
                return() { trace += "close "; if (closeThrows) throw new Error("close"); return { done: true }; }
            };
        }
    };
}
for (const value of source()) { break; }
console.log("break", trace);
trace = "";
function earlyReturn() { for (const value of source()) { return value; } }
console.log("return", earlyReturn(), trace);
trace = "";
try { for (const value of source(true)) { throw new Error("body"); } }
catch (error) { console.log("body-wins", error.message, trace); }
trace = "";
try { for (const value of source(true)) { break; } }
catch (error) { console.log("close-wins", error.message, trace); }
trace = "";
for (const value of source()) { trace += value + " "; }
console.log("exhausted", trace);
trace = "";
const [first] = source();
console.log("destructure", first, trace);
trace = "";
const broken = { [Symbol.iterator]() { return { next() { throw new Error("next"); }, return() { trace += "close"; return {}; } }; } };
try { for (const value of broken) {} } catch (error) { console.log("next-error", error.message, trace); }
