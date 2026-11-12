// M1: TDZ applies to reads, writes, typeof, and shadowed outer bindings.
const value = "outer";
try { console.log(value); let value = "inner"; }
catch (error) { console.log("shadow-read", error.name); }
try { local = 1; let local; }
catch (error) { console.log("write", error.name); }
try { console.log(typeof local); let local; }
catch (error) { console.log("typeof-tdz", error.name); }
try { let self = self; console.log(self); }
catch (error) { console.log("initializer", error.name); }
try { const closure = () => later; closure(); let later = 1; }
catch (error) { console.log("closure-early", error.name); }
const read = () => later;
let later = 4;
let empty;
console.log("initialized", read(), empty, value, typeof genuinelyAbsentRoadmapName);
