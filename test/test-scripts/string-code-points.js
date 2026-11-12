// M3: Code points, UTF-16 indices, and normalization forms.
const text = "A😀B";
console.log("points", text.codePointAt(0), text.codePointAt(1), text.codePointAt(2), text.codePointAt(4), text.codePointAt(-1));
console.log("construct", String.fromCodePoint(65, 0x1F600, 66) === text, String.fromCharCode(0xD83D, 0xDE00) === "😀");
console.log("lone", "\uD800".codePointAt(0), String.fromCodePoint(0xD800).charCodeAt(0));
for (const invalid of [-1, 1.5, 0x110000, NaN]) {
    try { String.fromCodePoint(invalid); } catch (error) { console.log("invalid-point", error.name); }
}
console.log("normalization", "e\u0301".normalize() === "é", "é".normalize("NFD") === "e\u0301");
console.log("compatibility", "\uFB01".normalize("NFKC"), "\uFB01".normalize("NFKD"));
try { "x".normalize("invalid"); } catch (error) { console.log("invalid-form", error.name); }
