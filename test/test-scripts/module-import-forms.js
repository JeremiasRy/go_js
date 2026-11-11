// M5: Module goal. Default/named/aliased/namespace imports and re-exports.
import describe, { number as renamed, title, add } from "./test-module/roadmap/exports.js";
import * as namespace from "./test-module/roadmap/exports.js";
import forwarded, * as reexports from "./test-module/roadmap/reexports.js";
import "./test-module/roadmap/side-effect.js";

console.log("imports", describe(title), renamed, add(2, 3), namespace.default === describe);
console.log("reexports", forwarded === describe, reexports.describe === describe, reexports.renamedNumber, reexports.add(3, 4), reexports.counter.count);
console.log("ambiguous-star", "increment" in reexports, "readLocal" in reexports, typeof reexports.bump);
console.log("namespace", Object.getPrototypeOf(namespace) === null, Object.isExtensible(namespace), Object.keys(namespace).join(","));
const descriptor = Object.getOwnPropertyDescriptor(namespace, "number");
console.log("descriptor", descriptor.value, descriptor.writable, descriptor.enumerable, descriptor.configurable);
try { namespace.number = 9; } catch (error) { console.log("namespace-write", error.name); }
try { delete namespace.number; } catch (error) { console.log("namespace-delete", error.name); }
console.log("side-effect", globalThis.roadmapModuleLoads);
