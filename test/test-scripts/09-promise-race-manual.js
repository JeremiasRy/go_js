// Probe 09 — Promise.race between manually constructed promises.
//
// Node 24 baseline: prints "winner: fast".
// go_js observed (2026-11-14): identical output. Manual `new Promise` values
// with timer-based resolution are the reliable subset of the Promise API.

const slow = new Promise((resolve) => setTimeout(() => resolve('slow'), 80));
const fast = new Promise((resolve) => setTimeout(() => resolve('fast'), 10));

console.log('winner:', await Promise.race([slow, fast]));
