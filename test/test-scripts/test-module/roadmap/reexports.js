// Shared module fixture: explicit exports resolve separately from star conflicts.
export { default, default as describe, number as renamedNumber, increment as bump } from "./exports.js";
export * from "./exports.js";
export * from "./counter.js";
export * as counter from "./counter.js";
