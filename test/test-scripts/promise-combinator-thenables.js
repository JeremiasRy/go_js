Promise.all([{ then(resolve) { resolve(1); } }, 2]).then(v => console.log(v.join(",")));
console.log("sync");
