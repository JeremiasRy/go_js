// M1: Untagged templates use cooked text and left-to-right substitutions.
const cooked = `a\nb\t\u0041\u{1F600}\`\\`;
console.log("cooked", cooked === "a\nb\tA😀`\\", cooked.length);
console.log("escaped-substitution", `\${literal}` === "${literal}");
console.log("line-continuation", `left\
right` === "leftright");
console.log("multiline", `a
b` === "a\nb");
let trace = "";
function value(label) { trace += label; return label; }
console.log("order", `${value("a")}:${value("b")}`, trace);
console.log("coercion", `${null}|${undefined}|${false}|${-1}`);
