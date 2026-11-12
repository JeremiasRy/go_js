// M3: ReplaceAll handles every non-overlapping match, including boundaries.
console.log("all", "banana".replaceAll("a", "X"), "aaaa".replaceAll("aa", "b"));
console.log("empty-search", "ab".replaceAll("", "-"), "".replaceAll("", "-"));
console.log("substitutions", "aba".replaceAll("a", "[$$][$&][$`][$']"));
let trace = "";
console.log("callback", "ababa".replaceAll("ba", (match, index, original) => {
    trace += `${match}:${index}:${original} `;
    return index;
}), trace);
console.log("missing", "abc".replaceAll("z", "x"));
console.log("utf16-boundaries", "😀".replaceAll("", "-").length);
