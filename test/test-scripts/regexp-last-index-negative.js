const r = /a/g;
r.lastIndex = -1;
const m = r.exec("aaa");
console.log(m.index, r.lastIndex);
