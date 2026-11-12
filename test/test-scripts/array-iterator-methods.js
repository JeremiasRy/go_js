const e = [1, 2].entries();
console.log(JSON.stringify(e.next()), JSON.stringify(e.next()), JSON.stringify(e.next()));
console.log([...["a", "b"].keys()].join(","));
console.log([...["a", "b"].values()].join(","));
