// The two star-export sources both define increment, making it ambiguous.
import { increment } from "../../test-scripts/test-module/roadmap/reexports.js";
export { increment };
