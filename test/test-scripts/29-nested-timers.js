// Probe 29 — nested timers resolving a manual promise.
//
// Node 24 baseline:
//   step1
//   step2
//   step3
//   done
//
// go_js observed (2026-11-14): identical output. Nested timer callbacks and a
// manual promise resolved from the innermost callback are reliable.

let finish;
const donePromise = new Promise((resolve) => {
  finish = resolve;
});

setTimeout(() => {
  console.log('step1');
  setTimeout(() => {
    console.log('step2');
    setTimeout(() => {
      console.log('step3');
      finish();
    }, 20);
  }, 20);
}, 20);

await donePromise;
console.log('done');
