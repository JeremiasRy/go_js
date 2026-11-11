// M6: Run in Script and Module goals to compare global-property creation.
console.log("before", roadmapGlobalVar);
var roadmapGlobalVar = 1;
let roadmapGlobalLet = 2;
console.log("properties", Object.hasOwn(globalThis, "roadmapGlobalVar"), Object.hasOwn(globalThis, "roadmapGlobalLet"));
const descriptor = Object.getOwnPropertyDescriptor(globalThis, "roadmapGlobalVar");
console.log("descriptor", descriptor?.writable, descriptor?.enumerable, descriptor?.configurable);
roadmapGlobalVar = 3;
console.log("binding", roadmapGlobalVar, globalThis.roadmapGlobalVar, roadmapGlobalLet);
globalThis.roadmapGlobalVar = 4;
console.log("property-write", roadmapGlobalVar, globalThis.roadmapGlobalVar);
console.log("delete-property", Reflect.deleteProperty(globalThis, "roadmapGlobalVar"), roadmapGlobalVar);
