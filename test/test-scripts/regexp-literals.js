// M6: Literals/constructors, escaped patterns, flags, and syntax validation.
const literal = /a\/b/i;
const constructed = new RegExp("a/b", "i");
console.log("equivalent", literal.test("A/B"), constructed.test("A/B"), literal.source, literal.flags);
console.log("escapes", /\d+/.test("123"), new RegExp("\\d+").test("123"), /\u{1F600}/u.test("😀"));
console.log("character-class", /[a-z]+/i.test("ABC"), /[^0-9]/.test("123"));
function make() { return /a/g; }
const first = make();
const second = make();
first.lastIndex = 3;
console.log("fresh-literals", first !== second, second.lastIndex);
console.log("constructor-identity", RegExp(first) === first, new RegExp(first) !== first, new RegExp(first).flags);
for (const pair of [["(", ""], ["a", "gg"], ["a", "z"], ["a", "uv"]]) {
    try { new RegExp(pair[0], pair[1]); } catch (error) { console.log("invalid", error.name); }
}
