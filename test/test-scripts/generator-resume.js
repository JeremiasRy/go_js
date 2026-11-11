// M4: Suspended locals, input values, bare yield, return, and completion.
function* values(start) {
    let local = start;
    local += yield local;
    const next = yield;
    return local + next;
}
const generator = values(2);
for (const argument of [999, 3, 4, 5]) {
    const result = generator.next(argument);
    console.log("next", result.value, result.done);
}
const returned = generator.return(7);
console.log("return-completed", returned.value, returned.done);
try { generator.throw("completed-error"); } catch (reason) { console.log("throw-completed", reason); }
const first = values(10);
const second = values(20);
console.log("independent", first.next().value, second.next().value, first.next(1).done, second.next(2).done);
