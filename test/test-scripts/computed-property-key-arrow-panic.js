let log = [];
const o = { [(() => { log.push("obj"); return "b"; })()]: 2 };
console.log(o.b, log.join(","));
