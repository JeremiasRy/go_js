const bad = { [Symbol.iterator]() { return { next() { return 1; } }; } };
Promise.all(bad).catch(e => console.log(e.name));
