const m = new Map([["a", 1]]);
m.forEach(function () { console.log(this === undefined, typeof this); });
