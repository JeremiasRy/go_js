// M1: Stable error classes/messages and catchable language exceptions.
for (const Constructor of [Error, TypeError, ReferenceError, RangeError, SyntaxError]) {
    const error = new Constructor("controlled message");
    console.log("constructed", error.name, error.message, error instanceof Error, error instanceof Constructor);
}
console.log("empty-message", new Error().message === "");
const operations = [
    () => { const value = 1; value(); },
    () => missingRoadmapBinding,
    () => new Array(-1),
    () => JSON.parse("{")
];
for (const operation of operations) {
    try {
        operation();
        console.log("missing-error");
    } catch (error) {
        console.log("caught", error.name, error instanceof Error);
    }
}
function inner() { throw new TypeError("nested"); }
try { inner(); } catch (error) { console.log("unwound", error.name, error.message); }
console.log("continued");
