// M3: Slice and substring use different negative/reversed index rules.
const text = "abcdef";
for (const pair of [[1, 4], [-3, -1], [4, 1], [-99, 99], [2.9, 4.9], [NaN, 3], [2, Infinity]]) {
    console.log("range", text.slice(pair[0], pair[1]), text.substring(pair[0], pair[1]));
}
console.log("omitted-end", text.slice(2), text.substring(2), text.slice(), text.substring());
const astral = "A😀B";
console.log("code-units", astral.slice(1, 2) === "\uD83D", astral.substring(2, 3) === "\uDE00", astral.slice(1, 3) === "😀");
console.log("generic", String.prototype.slice.call(12345, 1, 3));
try { String.prototype.substring.call(null, 0); } catch (error) { console.log("null-receiver", error.name); }
