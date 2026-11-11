// M3: Search equality, fromIndex conversion, holes, and relative indexing.
const array = [1, NaN, 2, 1, undefined];
console.log("equality", array.includes(NaN), array.indexOf(NaN), array.lastIndexOf(NaN));
console.log("positions", array.indexOf(1), array.indexOf(1, 1), array.indexOf(1, -2), array.lastIndexOf(1), array.lastIndexOf(1, -3));
console.log("bounds", array.includes(1, Infinity), array.indexOf(1, -Infinity), array.lastIndexOf(1, -Infinity));
console.log("holes", [,].includes(undefined), [,].indexOf(undefined), [,].lastIndexOf(undefined));
console.log("at", array.at(0), array.at(-1), array.at(-2), array.at(99), array.at(-99), array.at(1.9));
console.log("find", array.find(value => value === 2), array.findIndex(value => value === 2), array.findIndex(value => value === 9));
const object = {};
console.log("identity", [object].includes(object), [object].includes({}));
