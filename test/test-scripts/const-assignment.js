// M1: Const protects the binding, including captured bindings and updates.
const answer = 1;
for (const assign of [() => { answer = 2; }, () => { answer += 1; }, () => answer++]) {
    try { assign(); console.log("missing-error"); }
    catch (error) { console.log("const", error.name, answer); }
}
const object = { value: 1 };
object.value = 2;
console.log("mutable-contents", object.value);
const { value: captured } = object;
function reassign() { captured = 3; }
try { reassign(); } catch (error) { console.log("captured", error.name, captured); }
for (const value of [1, 2]) {
    try { value = 0; } catch (error) { console.log("iteration", error.name, value); }
}
