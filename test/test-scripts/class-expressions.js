// M5: Named/anonymous expression bindings, inferred names, and class TDZ.
const Bound = class Internal {
    static same() { return Internal === Bound; }
    name() { return Internal.name; }
};
console.log("named", Bound.name, Bound.same(), new Bound().name(), typeof Internal);
const Anonymous = class { value() { return 7; } };
console.log("inferred", Anonymous.name, new Anonymous().value());
function make(value) { return class { read() { return value; } }; }
const First = make(1);
const Second = make(2);
console.log("closure", new First().read(), new Second().read(), First !== Second);
let trace = "";
function base() { trace += "extends "; return class {}; }
const Computed = class extends base() { [(() => { trace += "key "; return "read"; })()]() { return 3; } };
console.log("evaluation", new Computed().read(), trace);
try { console.log(Later); class Later {} }
catch (error) { console.log("declaration-tdz", error.name); }
try { const Self = class Self extends Self {}; console.log(Self); }
catch (error) { console.log("expression-tdz", error.name); }
