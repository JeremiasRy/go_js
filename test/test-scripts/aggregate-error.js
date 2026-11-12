const a = new AggregateError([1, 2], "m");
console.log(a.name, a.message, a.errors.join(","), a instanceof Error);
