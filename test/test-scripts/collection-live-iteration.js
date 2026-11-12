const m = new Map([["a", 1], ["b", 2]]);
const seenMap = [];
for (const [k] of m) { seenMap.push(k); if (k === "a") { m.set("c", 3); } if (k === "c") { m.delete("b"); } }
console.log(seenMap.join(","), m.size);
const s = new Set([1, 2]);
const seenSet = [];
for (const v of s) { seenSet.push(v); if (v === 1) { s.add(3); } }
console.log(seenSet.join(","), s.size);
