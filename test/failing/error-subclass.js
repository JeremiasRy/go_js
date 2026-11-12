class E extends Error { constructor(m) { super(m); this.name = "E"; } }
const e = new E("x");
console.log(e instanceof E, e instanceof Error, e.name, e.message, String(e));
class T extends TypeError {}
const t = new T("y");
console.log(t.name, t instanceof TypeError, t instanceof Error);
