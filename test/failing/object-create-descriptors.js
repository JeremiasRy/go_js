const o = Object.create({ p: 1 }, { x: { value: 2, enumerable: true }, y: { get() { return 3; }, enumerable: true } });
console.log(o.p, o.x, o.y, Object.keys(o).filter(k => k !== "y").join(","), Object.getPrototypeOf(o).p);
