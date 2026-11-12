const o = { a: 1 };
Object.preventExtensions(o);
try { Object.defineProperty(o, "c", { value: 3 }); } catch (e) { console.log(e.name); }
console.log(Object.isExtensible(o), "c" in o);
