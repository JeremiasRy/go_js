// M6: Forwarding traps preserve receivers and ordinary reflection behavior.
let trace = "";
const target = { value: 1, get doubled() { return this.value * 2; } };
const proxy = new Proxy(target, {
    get(object, key, receiver) { trace += "get:" + key + " "; return Reflect.get(object, key, receiver); },
    set(object, key, value, receiver) { trace += "set:" + key + " "; return Reflect.set(object, key, value, receiver); },
    has(object, key) { trace += "has:" + key + " "; return Reflect.has(object, key); },
    deleteProperty(object, key) { trace += "delete:" + key + " "; return Reflect.deleteProperty(object, key); },
    ownKeys(object) { trace += "keys "; return Reflect.ownKeys(object); },
    getOwnPropertyDescriptor(object, key) { trace += "descriptor:" + key + " "; return Reflect.getOwnPropertyDescriptor(object, key); },
    defineProperty(object, key, descriptor) { trace += "define:" + key + " "; return Reflect.defineProperty(object, key, descriptor); },
    getPrototypeOf(object) { trace += "get-prototype "; return Reflect.getPrototypeOf(object); },
    setPrototypeOf(object, prototype) { trace += "set-prototype "; return Reflect.setPrototypeOf(object, prototype); },
    isExtensible(object) { trace += "extensible "; return Reflect.isExtensible(object); },
    preventExtensions(object) { trace += "prevent "; return Reflect.preventExtensions(object); }
});
proxy.value = 3;
console.log("receiver", proxy.doubled, trace);
trace = "";
console.log("membership", "value" in proxy, delete proxy.missing, Object.keys(proxy).join(","), trace);
trace = "";
Object.defineProperty(proxy, "extra", { value: 4, configurable: true });
console.log("reflection", Object.getOwnPropertyDescriptor(proxy, "extra").value, Object.getPrototypeOf(proxy) === Object.prototype, Reflect.setPrototypeOf(proxy, null), Reflect.isExtensible(proxy), Reflect.preventExtensions(proxy), trace);
function Box(value) { if (new.target) { this.value = value; this.target = new.target; } else return this.offset + value; }
const callable = new Proxy(Box, {
    apply(fn, receiver, argumentsList) { console.log("apply-trap", argumentsList[0]); return Reflect.apply(fn, receiver, argumentsList); },
    construct(fn, argumentsList, newTarget) { console.log("construct-trap", newTarget === callable); return Reflect.construct(fn, argumentsList, newTarget); }
});
console.log("call", callable.call({ offset: 10 }, 2));
const instance = new callable(7);
console.log("construct", instance.value, instance.target === callable, instance instanceof Box);
