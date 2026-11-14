// Probe 28 — callback-only retry with backoff.
//
// Node 24 baseline:
//   try 1
//   fail 1
//   try 2
//   fail 2
//   try 3
//   SUCCESS after 3
//
// go_js observed (2026-11-14): process exits with status 1:
//   runtime error: TypeError: Value is not callable
//
// Pure callbacks + timers, no promises, no async functions — still broken.

let tries = 0;

function attempt() {
  tries++;
  console.log('try ' + tries);
  setTimeout(() => {
    if (tries < 3) {
      console.log('fail ' + tries);
      setTimeout(attempt, 20);
    } else {
      console.log('SUCCESS after ' + tries);
      clearInterval(hb);
    }
  }, 10);
}

attempt();

const hb = setInterval(() => {}, 20);
