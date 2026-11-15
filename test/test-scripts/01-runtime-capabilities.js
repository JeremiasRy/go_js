// Probe 01 — runtime capability check.
//
// Node 24 baseline:
//   AbortController is a function; the timer and the abortable fetch succeed.
//
// go_js observed (2026-11-14):
//   setTimeout/setInterval/fetch/performance/queueMicrotask exist, but
//   AbortController === "undefined", so the abortable fetch fails with
//   "ReferenceError: AbortController is not defined".

const caps = {
  setTimeout: typeof setTimeout,
  setInterval: typeof setInterval,
  clearTimeout: typeof clearTimeout,
  AbortController: typeof AbortController,
  fetch: typeof fetch,
  performance: typeof performance,
  queueMicrotask: typeof queueMicrotask
};
console.log('caps:', JSON.stringify(caps));

const t0 = Date.now();
await new Promise((resolve) => setTimeout(resolve, 50));
console.log('timer ok:', Date.now() - t0 >= 50);

try {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 4000);
  const res = await fetch('https://httpbin.org/get', { signal: controller.signal });
  clearTimeout(timer);
  console.log('fetch ok:', res.status);
} catch (e) {
  console.log('fetch failed:', e.name + ':', e.message);
}
