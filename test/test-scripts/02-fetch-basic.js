// Probe 02 — plain fetch against public endpoints (three requests).
//
// Node 24 baseline: all three requests return HTTP 200.
// Output is normalized so it is deterministic across runtimes.

async function fetchOne(url) {
  const res = await fetch(url);
  const text = await res.text();
  const body = JSON.parse(text);
  let detail = '';
  if (url.indexOf('/todos/') >= 0) {
    detail = 'id=' + body.id;
  } else if (url.indexOf('/delay/') >= 0) {
    detail = 'delayed=' + (res.status === 200);
  } else {
    detail = 'uuid=' + typeof body.uuid;
  }
  return 'status=' + res.status + ' ' + detail;
}

const urls = [
  'https://jsonplaceholder.typicode.com/todos/1',
  'https://httpbin.org/delay/1',
  'https://httpbin.org/uuid'
];

for (const url of urls) {
  console.log(url + ' ' + (await fetchOne(url)));
}
