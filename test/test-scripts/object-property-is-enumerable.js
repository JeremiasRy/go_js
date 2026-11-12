const o = { a: 1 };
Object.defineProperty(o, "b", { value: 2 });
console.log(o.propertyIsEnumerable("a"), o.propertyIsEnumerable("b"), o.propertyIsEnumerable("toString"));
