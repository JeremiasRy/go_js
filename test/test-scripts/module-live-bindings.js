// M5: Module goal. Imports/namespaces expose live read-only bindings.
import { count as importedCount, increment, readLocal } from "./test-module/roadmap/counter.js";
import * as counter from "./test-module/roadmap/counter.js";
import { readLocal as readExportLocal, mutable, increment as incrementExport } from "./test-module/roadmap/exports.js";

let count = 100;
console.log("initial", importedCount, counter.count, count, mutable);
increment();
counter.increment();
incrementExport();
console.log("updated", importedCount, counter.count, count, mutable);
console.log("module-scope", readLocal(), readExportLocal());
try { importedCount = 9; } catch (error) { console.log("import-write", error.name); }
try { counter.count++; } catch (error) { console.log("namespace-write", error.name); }
console.log("after-errors", importedCount, counter.count);
