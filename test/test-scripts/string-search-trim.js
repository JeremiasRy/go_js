// M3: Search positions, empty needles, Unicode whitespace, and case mapping.
const text = "ababa";
console.log("index", text.indexOf("ba"), text.indexOf("ba", 2), text.lastIndexOf("ba"), text.lastIndexOf("ba", 2));
console.log("empty-needle", text.indexOf("", 99), text.lastIndexOf("", -2));
console.log("positions", text.includes("ab", 2), text.startsWith("ba", 1), text.endsWith("ab", 2));
console.log("long-needle", "a".startsWith("abcd"), "a".endsWith("abcd"), "a".includes("abcd"));
console.log("clamp", text.startsWith("ab", -9), text.endsWith("ba", 99), text.includes("", 99));
const whitespace = "\uFEFF\u00A0 \ttext\n\r ";
console.log("trim", whitespace.trim(), whitespace.trimStart().endsWith("\n\r "), whitespace.trimEnd().startsWith("\uFEFF\u00A0"));
console.log("not-whitespace", "\u200Btext\u200B".trim().length);
console.log("case", "Straße".toUpperCase(), "ABC".toLowerCase(), "ΟΣ".toLowerCase());
try { text.includes(/a/); } catch (error) { console.log("regexp-needle", error.name); }
