// M6: RegExp/string integration, replacement groups, and well-known hooks.
console.log("match", "a1b22".match(/\d+/g).join(","), "a1".match(/(\d)/)[1]);
const searching = /b/g;
searching.lastIndex = 9;
console.log("search", "abc".search(searching), searching.lastIndex);
console.log("replace", "abc-12".replace(/(?<word>[a-z]+)-(\d+)/, "$2:$<word>:$&:$$"));
let trace = "";
console.log("callback", "a1b2".replace(/([a-z])(\d)/g, (match, letter, digit, index, original) => {
    trace += `${match}:${index}:${original} `;
    return digit + letter;
}), trace);
console.log("split-captures", "a1b2c".split(/(\d)/).join("|"), "a1b2c".split(/\d/, 2).join("|"));
const matching = /a/g;
matching.lastIndex = 1;
console.log("match-all", [..."aba".matchAll(matching)].map(match => match.index).join(","), matching.lastIndex);
console.log("empty-unicode", [..."😀".matchAll(/(?:)/gu)].map(match => match.index).join(","));
console.log("replace-all", "aba".replaceAll(/a/g, "x"));
for (const operation of [() => "aba".replaceAll(/a/, "x"), () => "aba".matchAll(/a/)]) {
    try { operation(); } catch (error) { console.log("global-required", error.name); }
}
const hooks = {
    flags: "g",
    [Symbol.match](text) { return "match:" + text; },
    [Symbol.search](text) { return text.length; },
    [Symbol.replace](text, replacement) { return text + ":" + replacement; },
    [Symbol.split](text) { return ["split", text]; },
    [Symbol.matchAll](text) { return [[text]][Symbol.iterator](); }
};
console.log("hooks", "abc".match(hooks), "abc".search(hooks), "abc".replace(hooks, "x"), "abc".replaceAll(hooks, "y"), "abc".split(hooks).join(":"), [..."abc".matchAll(hooks)][0][0]);
