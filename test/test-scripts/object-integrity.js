// M2: Extensibility, sealing, shallow freezing, and strict writes.
const limited = { value: 1 };
console.log("prevent", Object.preventExtensions(limited) === limited, Object.isExtensible(limited));
limited.extra = 2;
limited.value = 3;
console.log("existing-write", limited.value, Object.hasOwn(limited, "extra"));
const sealed = Object.seal({ value: 1 });
sealed.value = 2;
console.log("sealed", Object.isSealed(sealed), Object.isFrozen(sealed), delete sealed.value, sealed.value);
const frozen = Object.freeze({ child: { value: 1 } });
frozen.child.value = 2;
console.log("shallow", Object.isFrozen(frozen), Object.isFrozen(frozen.child), frozen.child.value);
function strictWrites() { "use strict"; frozen.child = {}; }
try { strictWrites(); } catch (error) { console.log("frozen-write", error.name); }
const array = Object.freeze([1, 2]);
try { array.push(3); } catch (error) { console.log("frozen-array", error.name, array.length); }
function truncate() { "use strict"; array.length = 0; }
try { truncate(); } catch (error) { console.log("frozen-length", error.name); }
let stored = 0;
const accessor = Object.freeze({ set value(next) { stored = next; } });
accessor.value = 9;
console.log("frozen-accessor", stored, Object.isFrozen(accessor));
