// M5: Module goal. Dynamic imports cache namespaces and reject failures.
const first = import("./test-module/roadmap/counter.js");
const second = import("./test-module/roadmap/counter.js");
console.log("promise", first instanceof Promise, first !== second);
const [one, two] = await Promise.all([first, second]);
one.increment();
console.log("identity", one === two, two.count);
let conversions = 0;
const coerced = await import({ toString() { conversions++; return "./test-module/roadmap/counter.js"; } });
console.log("specifier", coerced === one, conversions);
try { await import("./test-module/roadmap/intentionally-missing.js"); }
catch (error) { console.log("load-rejected", true); }
try { await import("../negative-scripts/fixtures/module-link-failure.js"); }
catch (error) { console.log("link-rejected", error.name); }
try { await import("../negative-scripts/fixtures/module-parse-failure.js"); }
catch (error) { console.log("parse-rejected", error.name); }
let failure;
try { await import("./test-module/roadmap/async-failure.js"); }
catch (error) { failure = error; console.log("evaluation-rejected", error.message); }
try { await import("./test-module/roadmap/async-failure.js"); }
catch (error) { console.log("cached-failure", error === failure); }
