// M2: Template identity, raw/cooked text, freezing, and tagged invalid escapes.
let previous;
function tag(strings, value) {
    console.log("template", strings[0] === "a\n", strings.raw[0] === "a\\n", strings[1], value);
    console.log("frozen", Object.isFrozen(strings), Object.isFrozen(strings.raw));
    console.log("identity", previous === strings);
    previous = strings;
    return value;
}
function site(value) { return tag`a\n${value}b`; }
console.log("return", site(1), site(2));
tag`a\n${3}b`;
const receiver = { tag(strings, value) { console.log("receiver", this === receiver, value); } };
let trace = "";
function substitution(value) { trace += value; return value; }
receiver.tag`${substitution(1)}${substitution(2)}`;
console.log("substitution-order", trace);
function invalid(strings) { console.log("invalid-cooked", strings[0] === undefined, strings.raw[0] === "\\unicode"); }
invalid`\unicode`;
