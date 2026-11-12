// M5: Module goal. Dependency evaluation completes before dependent code starts.
import { value, ready, trace } from "./test-module/roadmap/async-dependency.js";

console.log("dependency-ready", value, ready, trace.join(","));
const doubled = await Promise.resolve(value * 2);
console.log("top-level-await", doubled, await 3);
const again = await import("./test-module/roadmap/async-dependency.js");
console.log("cached", again.trace === trace, trace.join(","));
try { await import("../negative-scripts/fixtures/async-failure-dependent.js"); }
catch (error) { console.log("dependency-rejection", error.message, globalThis.roadmapFailedDependentRan === undefined); }
