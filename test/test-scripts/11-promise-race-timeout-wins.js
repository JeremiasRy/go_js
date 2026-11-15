// Probe 11 — Promise.race where the timeout wins.
//
// Node 24 baseline:
//   caught: timeout
//   late result ignored, no crash
//
// go_js observed (2026-11-14): identical output. This is the one async/race
// shape that survived: the async promise is never observed, because the
// timeout rejects first.

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function w() {
  await sleep(80);
  return 'late';
}

try {
  await Promise.race([
    w(),
    new Promise((_, reject) => setTimeout(() => reject(new Error('timeout')), 10))
  ]);
  console.log('unexpected: race resolved');
} catch (e) {
  console.log('caught:', e.message);
}

await sleep(120);
console.log('late result ignored, no crash');
