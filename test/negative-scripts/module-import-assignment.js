// Module goal; expected: execution-time TypeError for an immutable import.
import { value } from "./fixtures/module-exports.js";
value = 2;
