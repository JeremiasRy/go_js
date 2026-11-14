// Probe 02 — plain fetch against public endpoints (three requests).
//
// Node 24 baseline: all three requests return HTTP 200 (latencies vary).
//
// go_js observed (2026-11-14): all three requests returned HTTP 200.
// (The third endpoint in the original probe was example.com; it is replaced
// here with httpbin.org/uuid because example.com is unreachable from some
// sandboxes.)

async function tryUrl(url) {
  const t0 = Date.now();
  try {
    const res = await fetch(url);
    const text = await res.text();
    return {
      url,
      status: res.status,
      ms: Date.now() - t0,
      sample: text.slice(0, 80).replace(/\s+/g, ' ')
    };
  } catch (e) {
    return { url, error: e.name + ': ' + e.message, ms: Date.now() - t0 };
  }
}

const urls = [
  'https://jsonplaceholder.typicode.com/todos/1',
  'https://httpbin.org/delay/1',
  'https://httpbin.org/uuid'
];

for (const url of urls) {
  console.log(JSON.stringify(await tryUrl(url)));
}
