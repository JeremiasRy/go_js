import { labelB, readA } from "./cycle-b.js";
export const labelA = "A";
export function describeA() { return labelA + ":" + labelB + ":" + readA(); }
