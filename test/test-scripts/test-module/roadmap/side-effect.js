// Every entrypoint runs in isolation; repeated imports must increment only once.
globalThis.roadmapModuleLoads = (globalThis.roadmapModuleLoads || 0) + 1;
export const loads = globalThis.roadmapModuleLoads;
export const identity = {};
