export const trace = ["dependency:start"];
export const value = await Promise.resolve(21);
trace.push("dependency:ready");
export const ready = true;
