const a = [1, 2, 3];
Object.defineProperty(a, "length", { value: 1 });
console.log(a.length, a.join(","));
try { Object.defineProperty(a, "length", { value: 3 }); } catch (e) { console.log(e.name); }
