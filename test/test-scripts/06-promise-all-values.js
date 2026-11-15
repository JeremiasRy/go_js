// Probe 06 — Promise.all with plain values.
//
// Node 24 baseline: prints "Promise.all values: 1,2,3".
// go_js observed (2026-11-14): identical output. Promise.all itself exists;
// the failures in later probes depend on what is put into the array.

const result = await Promise.all([1, 2, 3]);
console.log('Promise.all values:', result.join(','));
