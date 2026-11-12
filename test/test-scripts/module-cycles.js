// M5: Module goal. Successful cycles defer reads; premature lexical reads fail.
import { labelA, describeA } from "./test-module/roadmap/cycle-a.js";
import { labelB, describeB } from "./test-module/roadmap/cycle-b.js";

console.log("cycle", labelA, labelB, describeA(), describeB());
const again = await import("./test-module/roadmap/cycle-a.js");
console.log("cached-cycle", again.describeA === describeA);
try { await import("../negative-scripts/fixtures/cycle-tdz-a.js"); }
catch (error) { console.log("cycle-tdz", error.name); }
