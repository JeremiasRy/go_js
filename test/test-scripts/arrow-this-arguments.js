// M2: Arrows inherit this/arguments and cannot be rebound or constructed.
const receiver = {
    value: 10,
    make(argument) {
        return () => `${this.value}:${arguments[0]}`;
    }
};
const read = receiver.make(7);
console.log("lexical", read(), read.call({ value: 99 }, 100), read.bind({ value: 88 })());
receiver.value = 11;
console.log("live-this", read());
function nested(value) { return (() => (() => arguments[0])())(); }
console.log("nested-arguments", nested(3));
function normal() { "use strict"; return this; }
const object = { normal };
const detached = object.normal;
console.log("normal-this", object.normal() === object, detached() === undefined);
try { new read(); } catch (error) { console.log("arrow-new", error.name); }
console.log("arrow-prototype", Object.hasOwn(read, "prototype"));
