const d = new Date(0);
console.log(String(d) === d.toString(), String(d).indexOf("1970") >= 0);
const bad = new Date(NaN);
console.log(String(bad), String(bad) === "Invalid Date");
