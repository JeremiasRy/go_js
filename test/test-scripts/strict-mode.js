// M1: Run in both goals to distinguish directive and implicit strictness.
function inheritedStrictness() { return this; }
function explicitlyStrict() { "use strict"; return this; }
console.log("inherited-this", inheritedStrictness() === undefined, inheritedStrictness() === globalThis);
console.log("strict-this", explicitlyStrict() === undefined, explicitlyStrict.call(null) === null);
function forbiddenAssignment() { "use strict"; roadmapUndeclaredStrictName = 1; }
try { forbiddenAssignment(); } catch (error) { console.log("strict-assignment", error.name); }
function inheritedAssignment() { roadmapUndeclaredGoalName = 2; }
try { inheritedAssignment(); console.log("goal-assignment", globalThis.roadmapUndeclaredGoalName); }
catch (error) { console.log("goal-assignment", error.name); }
delete globalThis.roadmapUndeclaredGoalName;
class StrictClass {
    method() { return this; }
    assign() { roadmapUndeclaredClassName = 3; }
}
const detached = new StrictClass().method;
console.log("class-this", detached() === undefined);
try { new StrictClass().assign(); } catch (error) { console.log("class-assignment", error.name); }
function unmapped(argument) { "use strict"; argument = 9; return arguments[0]; }
console.log("unmapped-arguments", unmapped(1));
