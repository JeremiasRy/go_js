// M2: Descriptor defaults, inspection, and redefinition invariants.
const object = {};
Object.defineProperty(object, "fixed", { value: 1 });
let descriptor = Object.getOwnPropertyDescriptor(object, "fixed");
console.log("defaults", descriptor.value, descriptor.writable, descriptor.enumerable, descriptor.configurable);
Object.defineProperties(object, {
    visible: { value: 2, writable: true, enumerable: true, configurable: true },
    hidden: { value: 3, writable: true }
});
console.log("keys", Object.keys(object).join(","), Object.getOwnPropertyNames(object).join(","));
Object.defineProperty(object, "visible", { value: 4 });
descriptor = Object.getOwnPropertyDescriptor(object, "visible");
console.log("preserved-flags", descriptor.value, descriptor.writable, descriptor.enumerable, descriptor.configurable);
Object.defineProperty(object, "fixed", { value: 1 });
for (const invalid of [{ value: 9 }, { writable: true }, { configurable: true }, { get() { return 1; } }]) {
    try { Object.defineProperty(object, "fixed", invalid); console.log("missing-error"); }
    catch (error) { console.log("redefine", error.name); }
}
try { Object.defineProperty(object, "mixed", { value: 1, get() { return 2; } }); }
catch (error) { console.log("mixed-descriptor", error.name); }
const descriptors = Object.getOwnPropertyDescriptors(object);
console.log("all-descriptors", descriptors.hidden.value, descriptors.fixed.configurable, Object.getOwnPropertyDescriptor(object, "absent"));
