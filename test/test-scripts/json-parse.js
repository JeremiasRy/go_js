// M3: JSON grammar, escapes, reviver order/deletion, and safe property creation.
const parsed = JSON.parse('{"text":"a\\n\\\"b","nested":{"value":2},"array":[true,null,3]}');
console.log("values", parsed.text === "a\n\"b", parsed.nested.value, parsed.array[0], parsed.array[1], parsed.array[2]);
console.log("primitives", JSON.parse("0"), JSON.parse("false"), JSON.parse("null"), Object.is(JSON.parse("-0"), -0));
let trace = "";
const revived = JSON.parse('{"a":1,"nested":{"b":2},"remove":3,"items":[4,5]}', function (key, value) {
    trace += (key === "" ? "root" : key) + " ";
    if (key === "remove" || key === "0") return undefined;
    return typeof value === "number" ? value * 10 : value;
});
console.log("reviver", trace, revived.a, revived.nested.b, "remove" in revived, 0 in revived.items, revived.items[1]);
console.log("root-deletion", JSON.parse("1", () => undefined));
const proto = JSON.parse('{"__proto__":{"polluted":true}}');
console.log("proto-key", Object.hasOwn(proto, "__proto__"), Object.getPrototypeOf(proto) === Object.prototype, proto.polluted);
console.log("surrogate", JSON.parse('"\\ud800"').charCodeAt(0));
for (const invalid of ["", "undefined", "01", "[1,]", '{"a":}', "true false", '"\n"']) {
    try { JSON.parse(invalid); } catch (error) { console.log("invalid", error.name); }
}
