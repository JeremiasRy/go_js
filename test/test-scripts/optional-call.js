// M1: Optional calls preserve method receivers and skip arguments.
let argumentsEvaluated = 0;
function argument() { argumentsEvaluated++; return 3; }
const object = { value: 10, method(amount) { return this.value + amount; } };
console.log("method", object.method?.(argument()), object?.method(argument()), argumentsEvaluated);
console.log("missing", object.missing?.(argument()), null?.method(argument()), argumentsEvaluated);
let callable;
console.log("optional-function", callable?.(argument()), argumentsEvaluated);
callable = value => value * 2;
console.log("function", callable?.(argument()), argumentsEvaluated);
try { ({ method: 7 }).method?.(argument()); }
catch (error) { console.log("non-callable", error.name, argumentsEvaluated); }
try { object?.missing(argument()); }
catch (error) { console.log("required-call", error.name, argumentsEvaluated); }
