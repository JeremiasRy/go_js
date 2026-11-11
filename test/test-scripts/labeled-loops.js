// M1: Resolve labels independently of the innermost loop/switch.
let trace = "";
outer: for (let i = 0; i < 3; i++) {
    for (let j = 0; j < 3; j++) {
        if (j === 1) continue outer;
        trace += `${i}${j} `;
    }
}
console.log("continue-outer", trace);
trace = "";
outer: for (let i = 0; i < 3; i++) {
    for (let j = 0; j < 3; j++) {
        trace += `${i}${j} `;
        if (i === 1) break outer;
        break;
    }
}
console.log("break-outer", trace);
block: { trace += "block "; break block; trace += "unreachable"; }
console.log("block-label", trace);
trace = "";
outer: for (let i = 0; i < 2; i++) {
    try { switch (i) { case 0: continue outer; default: break outer; } }
    finally { trace += i; }
}
console.log("label-finally", trace);
