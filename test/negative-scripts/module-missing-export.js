// Module goal; expected: linking-time SyntaxError.
import { missing } from "./fixtures/module-exports.js";
console.log(missing);
