// M6: Traps cannot contradict nonconfigurable properties or extensibility.
const fixed = {};
Object.defineProperty(fixed, "value", { value: 1, configurable: false, writable: false });
const closed = Object.preventExtensions({});
const operations = [
    ["get", () => new Proxy(fixed, { get() { return 2; } }).value],
    ["set", () => { new Proxy(fixed, { set() { return true; } }).value = 2; }],
    ["has", () => "value" in new Proxy(fixed, { has() { return false; } })],
    ["delete", () => delete new Proxy(fixed, { deleteProperty() { return true; } }).value],
    ["missing-key", () => Reflect.ownKeys(new Proxy(fixed, { ownKeys() { return []; } }))],
    ["duplicate-key", () => Reflect.ownKeys(new Proxy({}, { ownKeys() { return ["x", "x"]; } }))],
    ["extra-key", () => Reflect.ownKeys(new Proxy(closed, { ownKeys() { return ["extra"]; } }))],
    ["prototype", () => Object.getPrototypeOf(new Proxy(closed, { getPrototypeOf() { return null; } }))],
    ["extensible", () => Object.isExtensible(new Proxy(closed, { isExtensible() { return true; } }))],
    ["prevent", () => Object.preventExtensions(new Proxy({}, { preventExtensions() { return true; } }))],
    ["descriptor", () => Object.getOwnPropertyDescriptor(new Proxy(fixed, { getOwnPropertyDescriptor() { return undefined; } }), "value")],
    ["handler", () => new Proxy({}, null)]
];
for (const [label, operation] of operations) {
    try { operation(); console.log(label, "missing-error"); }
    catch (error) { console.log(label, error.name); }
}
const sameValue = new Proxy(fixed, { set() { return true; } });
console.log("same-value-allowed", Reflect.set(sameValue, "value", 1));
