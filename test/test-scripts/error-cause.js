const e = new Error("x", { cause: 42 });
console.log(e.message, e.cause);
