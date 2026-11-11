// M6: Captures, groups, backreferences, lookarounds, and match metadata.
const match = /(?<word>[a-z]+)-(\d+)(x)?/.exec("!abc-12?");
console.log("captures", match[0], match[1], match[2], match[3], match.groups.word);
console.log("metadata", match.index, match.input, match.length);
console.log("unmatched", /z/.exec("abc") === null, /z/.test("abc"));
console.log("backreferences", /([a-z]+)-\1/.test("abc-abc"), /([a-z]+)-\1/.test("abc-abd"));
console.log("named-backreference", /(?<word>[a-z]+):\k<word>/.test("same:same"));
console.log("lookaround", /a(?=b)/.exec("ab")[0], /a(?!b)/.test("ac"), /(?<=a)b/.exec("ab")[0], /(?<!a)b/.test("cb"));
console.log("unicode-properties", /^\p{Letter}+$/u.test("Ελληνικά"), /^\p{Number}+$/u.test("123"));
const astral = /😀(.)/u.exec("A😀B");
console.log("utf16-indices", astral.index, astral[0].length, astral[1]);
const optional = /(?<first>a)?b/.exec("b");
console.log("optional-group", optional[1], optional.groups.first, Object.getPrototypeOf(optional.groups) === null);
const plain = /a/;
plain.lastIndex = 99;
console.log("non-global", plain.exec("ba").index, plain.lastIndex);
