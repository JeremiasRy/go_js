// M1: Run as both a genuine Script and a Module, without a CommonJS wrapper.
let roadmapLexicalGlobal = 1;
const roadmapConstGlobal = 2;
function roadmapDeclaredFunction() { return 3; }
console.log("top-level-this", this === globalThis, this === undefined);
console.log("lexical-properties", Object.hasOwn(globalThis, "roadmapLexicalGlobal"), Object.hasOwn(globalThis, "roadmapConstGlobal"));
console.log("function-property", Object.hasOwn(globalThis, "roadmapDeclaredFunction"));
console.log("binding", roadmapLexicalGlobal, roadmapConstGlobal, roadmapDeclaredFunction());
console.log("constants", globalThis.undefined === undefined, globalThis.NaN !== globalThis.NaN, globalThis.Infinity === Infinity);
globalThis.roadmapExplicitGlobal = 4;
console.log("explicit-global", roadmapExplicitGlobal, globalThis.roadmapExplicitGlobal);
console.log("delete-global", delete globalThis.roadmapExplicitGlobal, typeof roadmapExplicitGlobal);
