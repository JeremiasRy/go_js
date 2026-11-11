// M2: Symbol keys remain distinct and are absent from string enumeration.
const first = Symbol("same");
const second = Symbol("same");
const object = { plain: 1, [first]: 2, [second]: 3 };
console.log("keys", object[first], object[second], Object.keys(object).join(","), Object.getOwnPropertyNames(object).join(","));
const symbols = Object.getOwnPropertySymbols(object);
console.log("symbol-order", symbols.length, symbols[0] === first, symbols[1] === second);
let enumerated = "";
for (const key in object) { enumerated += key; }
console.log("for-in", enumerated);
const { [first]: selected, ...rest } = object;
console.log("rest", selected, rest[first], rest[second], Object.getOwnPropertySymbols(rest).length);
Object.defineProperty(object, first, { enumerable: false });
const copy = { ...object };
console.log("enumerability", copy[first], copy[second], Object.getOwnPropertySymbols(object).length);
console.log("delete", delete object[second], second in object);
