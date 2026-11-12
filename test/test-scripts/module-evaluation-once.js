// M5: Module goal. Canonicalized paths and dynamic imports share one instance.
import * as first from "./test-module/roadmap/side-effect.js";
import * as second from "./test-module/roadmap/./side-effect.js";

console.log("static-identity", first === second, first.identity === second.identity, first.loads, globalThis.roadmapModuleLoads);
const third = await import("./test-module/roadmap/side-effect.js");
const fourth = await import("./test-module/roadmap/../roadmap/side-effect.js");
console.log("dynamic-identity", first === third, third === fourth, globalThis.roadmapModuleLoads);
