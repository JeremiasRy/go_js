// M2: Literal property names, shorthand/methods, and computed-key ordering.
let trace = "";
function key(value) { trace += "key:" + value + " "; return value; }
function value(number) { trace += "value:" + number + " "; return number; }
const shorthand = 8;
const object = {
    [key("a")]: value(1),
    [key("a")]: value(2),
    "quoted-key": 3,
    4: 5,
    shorthand,
    ["read"]() { return this.a; }
};
console.log("properties", object.a, object["quoted-key"], object["4"], object.shorthand, object.read());
console.log("order", trace, Object.keys(object).join(","));
const symbol = Symbol("key");
console.log("symbol", ({ [symbol]: 9 })[symbol]);
const parent = { inherited: 1 };
const special = { __proto__: parent };
const ordinary = { ["__proto__"]: parent };
console.log("proto-literal", special.inherited, Object.hasOwn(special, "__proto__"), Object.hasOwn(ordinary, "__proto__"));
