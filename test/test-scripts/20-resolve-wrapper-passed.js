// Probe 20 — passing a resolve-wrapper as a callback parameter.
//
// Node 24 baseline: prints "B=y".
//
// go_js observed (2026-11-14): exits 0 but prints "B=[object Function]".
// The awaited promise resolved with the function itself instead of the string
// passed by callIt. This looks like argument/identity confusion in the VM's
// callback and promise handling.

function callIt(cb) {
  setTimeout(() => {
    cb('y');
  }, 20);
}

let finish;
const done = new Promise((resolve) => {
  finish = (v) => {
    resolve(v);
  };
});

callIt(finish);

const v = await done;
console.log('B=' + v);
