// M2: Accessor receivers, inherited setters, and thrown exceptions.
let trace = "";
const parent = {
    get doubled() { trace += "get "; return this.value * 2; },
    set doubled(next) { trace += "set "; this.value = next / 2; }
};
const child = Object.create(parent);
child.value = 3;
console.log("inherited-get", child.doubled, trace);
child.doubled = 10;
console.log("inherited-set", child.value, parent.value, Object.hasOwn(child, "doubled"), trace);
Object.defineProperty(child, "broken", { get() { throw new RangeError("getter"); }, set(value) { throw new TypeError("setter"); } });
try { console.log(child.broken); } catch (error) { console.log("get-error", error.name, error.message); }
try { child.broken = 1; } catch (error) { console.log("set-error", error.name, error.message); }
Object.defineProperty(child, "readonly", { get() { return 7; } });
function strictWrite() { "use strict"; child.readonly = 8; }
try { strictWrite(); } catch (error) { console.log("readonly", error.name, child.readonly); }
const descriptor = Object.getOwnPropertyDescriptor(parent, "doubled");
console.log("descriptor", typeof descriptor.get, typeof descriptor.set, descriptor.value, descriptor.enumerable);
