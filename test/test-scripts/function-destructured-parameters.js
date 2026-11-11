// M2: Recursive parameter patterns, defaults, aliases, and rest.
function describe({ name: label = "guest", address: { city = "unknown" } = {} } = {}, [first = 1, ...rest] = []) {
    return `${label}:${city}:${first}:${rest.join(",")}`;
}
console.log("empty", describe());
console.log("provided", describe({ name: "Ada", address: { city: "London" } }, [2, 3, 4]));
console.log("undefined", describe({ name: undefined }, [undefined, 9]));
console.log("null-value", describe({ name: null }, [null]));
try { describe(null); } catch (error) { console.log("null-pattern", error.name); }
function rest({ first, ...remaining }, ...[{ value = 7 } = {}]) {
    return `${first}:${Object.keys(remaining).join(",")}:${value}`;
}
console.log("rest", rest({ first: 1, second: 2 }), rest({ first: 3 }, { value: 4 }));
console.log("length", describe.length, (({ a }, [b]) => a + b).length);
