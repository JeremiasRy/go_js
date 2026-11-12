// M1: Ordinary property operations on nullish receivers throw.
for (const value of [null, undefined]) {
    for (const operation of [
        () => value.name,
        () => value["name"],
        () => { value.name = 1; },
        () => value.method()
    ]) {
        try { operation(); console.log("missing-error"); }
        catch (error) { console.log("nullish", error.name); }
    }
}
console.log("missing-property", ({}).missing);
console.log("boxed", "abc".length, (3).missing, true.missing);
let keys = 0;
try { null[(keys++, "x")]; } catch (error) { console.log("computed-key", keys, error.name); }
