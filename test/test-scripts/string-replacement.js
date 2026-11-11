// M3: String-search replacement substitutions, callback arguments, and split.
console.log("first-only", "banana".replace("a", "X"));
console.log("substitutions", "abc".replace("b", "[$$][$&][$`][$']"));
let calls = 0;
console.log("callback", "ababa".replace("ba", (match, index, original) => {
    calls++;
    console.log("callback-arguments", match, index, original);
    return null;
}), calls);
console.log("no-captures", "abc".replace("b", "$1"));
console.log("empty-search", "ab".replace("", "-"), "undefined".replace(undefined, "value"));
console.log("split", "a,b,c".split(",", 2).join("|"), "abc".split(undefined).length, "abc".split(",", 0).length);
console.log("empty-split", "".split("").length, "".split(",").length, "😀".split("").length);
console.log("split-limit", "a,b,c".split(",", -1).join("|"));
