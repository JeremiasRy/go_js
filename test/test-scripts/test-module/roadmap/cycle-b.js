import { labelA } from "./cycle-a.js";
export const labelB = "B";
export function readA() { return labelA; }
export function describeB() { return labelB + ":" + labelA; }
