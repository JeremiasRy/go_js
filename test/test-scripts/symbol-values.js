// M2: Symbol identity, registry, explicit conversion, and primitive errors.
const first = Symbol("same");
const second = Symbol("same");
console.log("identity", first === first, first === second, typeof first, first.description);
const registered = Symbol.for("roadmap");
console.log("registry", registered === Symbol.for("roadmap"), Symbol.keyFor(registered), Symbol.keyFor(first));
console.log("description", Symbol().description, Symbol(3).description, String(first));
for (const operation of [() => new Symbol("x"), () => Symbol.keyFor("x"), () => +first, () => "" + first, () => `${first}`]) {
    try { operation(); console.log("missing-error"); }
    catch (error) { console.log("symbol-error", error.name); }
}
console.log("boxed", Object(first).valueOf() === first, !!first);
