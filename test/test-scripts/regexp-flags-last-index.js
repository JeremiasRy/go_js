// M6: Stateful g/y, canonical flags, Unicode code points, and match indices.
const global = /a/g;
let match = global.exec("aba");
console.log("global-first", match.index, global.lastIndex);
match = global.exec("aba");
console.log("global-second", match.index, global.lastIndex);
console.log("global-end", global.exec("aba") === null, global.lastIndex);
const sticky = /a/y;
sticky.lastIndex = 1;
console.log("sticky", sticky.exec("baac").index, sticky.lastIndex, sticky.exec("baac").index, sticky.lastIndex);
console.log("sticky-failure", sticky.test("baac"), sticky.lastIndex);
console.log("flags", new RegExp(".", "yusmigd").flags);
console.log("matching", /^b/m.test("a\nb"), /a/i.test("A"), /a.b/s.test("a\nb"));
console.log("unicode", /./.exec("😀")[0].length, /./u.exec("😀")[0].length, /./u.unicode);
const indexed = /(?<letter>a)(b)/d.exec("!ab");
console.log("indices", indexed.indices[0].join(","), indexed.indices[1].join(","), indexed.indices.groups.letter.join(","));
const empty = /(?:)/g;
empty.exec("a");
console.log("exec-empty-no-advance", empty.lastIndex);
