// M1: Strings expose UTF-16 code units, including unpaired surrogates.
const astral = "A😀B";
console.log("lengths", astral.length, "😀".length, "e\u0301".length, "".length);
console.log("units", astral.charCodeAt(0), astral.charCodeAt(1), astral.charCodeAt(2), astral.charCodeAt(3));
console.log("index", astral[1] === "\uD83D", astral[2] === "\uDE00", astral[4] === undefined);
console.log("charAt", astral.charAt(1) === "\uD83D", astral.charAt(-1) === "", astral.charAt(99) === "");
console.log("out-of-range-code", Number.isNaN(astral.charCodeAt(-1)), Number.isNaN("".charCodeAt(0)));
const lone = "\uD800";
console.log("lone", lone.length, lone.charCodeAt(0), lone[0] === lone);
console.log("fractional-index", astral.charAt(1.9) === "\uD83D", astral.charCodeAt());
