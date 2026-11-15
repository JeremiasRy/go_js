// Full task-queue stress lab — Node 24 baseline.
//
// Patterns: aging priority queue (binary heap), 3-worker pool, token-bucket
// rate limiter, retry with exponential backoff + jitter, circuit breaker,
// Promise.race timeouts, fetch against live endpoints, dead-letter queue,
// ASCII timeline and queue-depth sparkline.
//
// Node 24: completes in ~3-4 seconds with most tasks completed and a few in
// the dead-letter queue.
//
// go_js (2026-11-14):
//   1) With `for(;;)` in the heap/loops: compiler SIGSEGV in withAstTracking.
//   2) After rewriting to `while (true)`: VM panic in promiseReaction:
//      "interface conversion: object.Object is *native.RejectFunc,
//       not *native.ObjPromise" (triggered by the Promise.all worker barrier).
//   The lab never completed under go_js.
//
// The code below is the Node-valid version (while(true) loops, awaited
// Promise.all). Probe 17 isolates the mapLimit/Promise.all failure.
//
// Run: node lab/task-queue-lab.js   (or: npm run lab:node24)

'use strict';

const t0 = performance.now();
const events = [];

function rel() { return Math.round(performance.now() - t0); }
function log(evt, msg) {
  const t = rel();
  events.push({ t: t, evt: evt, msg: msg });
  console.log(String(t).padStart(5, ' ') + 'ms  ' + evt.padEnd(9, ' ') + ' ' + msg);
}
function sleep(ms) { return new Promise(function (resolve) { setTimeout(resolve, ms); }); }
function trim(s, n) { s = String(s); return s.length <= n ? s : s.slice(0, n - 1) + '~'; }

console.log('=== TASK QUEUE STRESS LAB ===');
console.log('aging-priority queue | 3-worker pool | token-bucket net limiter | retry+backoff | circuit breaker');
console.log('runtime: timers=yes fetch=yes AbortController=NO (timeouts via Promise.race; timed-out work keeps running)');
console.log('');

// ---------- binary min-heap ----------
class MinHeap {
  constructor(cmp) { this.items = []; this.cmp = cmp; }
  get size() { return this.items.length; }
  push(x) {
    const a = this.items;
    a.push(x);
    let i = a.length - 1;
    while (i > 0) {
      const p = (i - 1) >> 1;
      if (this.cmp(a[i], a[p]) >= 0) break;
      const tmp = a[i]; a[i] = a[p]; a[p] = tmp;
      i = p;
    }
  }
  pop() {
    const a = this.items;
    if (a.length === 0) return undefined;
    const top = a[0];
    const last = a.pop();
    if (a.length > 0) { a[0] = last; this.sink(0); }
    return top;
  }
  sink(i) {
    const a = this.items;
    const n = a.length;
    while (true) {
      const l = 2 * i + 1;
      const r = l + 1;
      let m = i;
      if (l < n && this.cmp(a[l], a[m]) < 0) m = l;
      if (r < n && this.cmp(a[r], a[m]) < 0) m = r;
      if (m === i) break;
      const tmp = a[i]; a[i] = a[m]; a[m] = tmp;
      i = m;
    }
  }
  rebuild() {
    for (let i = (this.items.length >> 1) - 1; i >= 0; i--) this.sink(i);
  }
}

// ---------- priority queue with aging (anti-starvation) ----------
// priority: 0 = high, 1 = normal, 2 = low. Waiting tasks get promoted
// gradually, so low-priority work cannot starve under a high-priority flood.
class AgingPriorityQueue {
  constructor(agingPerSec) {
    this.agingPerSec = agingPerSec;
    this.seq = 0;
    this.heap = new MinHeap(function (a, b) {
      return (a.key - b.key) || (a.seq - b.seq);
    });
  }
  refresh() {
    const now = performance.now();
    for (const t of this.heap.items) {
      const ageSec = (now - t.enqueuedAt) / 1000;
      t.key = t.priority - Math.min(t.priority, ageSec * this.agingPerSec);
    }
    this.heap.rebuild();
  }
  push(task) {
    task.seq = this.seq++;
    task.enqueuedAt = performance.now();
    task.key = task.priority;
    this.heap.push(task);
  }
  pop() {
    this.refresh();
    return this.heap.pop();
  }
  get size() { return this.heap.size; }
}

// ---------- token bucket rate limiter ----------
class TokenBucket {
  constructor(capacity, refillPerSec) {
    this.cap = capacity;
    this.tokens = capacity;
    this.rate = refillPerSec;
    this.last = performance.now();
    this.starved = 0;
  }
  refill() {
    const now = performance.now();
    this.tokens = Math.min(this.cap, this.tokens + ((now - this.last) / 1000) * this.rate);
    this.last = now;
  }
  async take() {
    while (true) {
      this.refill();
      if (this.tokens >= 1) { this.tokens -= 1; return; }
      this.starved++;
      const waitMs = Math.ceil(((1 - this.tokens) / this.rate) * 1000);
      await sleep(Math.max(5, waitMs));
    }
  }
}

// ---------- circuit breaker ----------
class CircuitBreaker {
  constructor(name, threshold, cooldownMs) {
    this.name = name;
    this.threshold = threshold;
    this.cooldownMs = cooldownMs;
    this.failures = 0;
    this.state = 'closed';
    this.openUntil = 0;
    this.probe = false;
  }
  allow() {
    const now = performance.now();
    if (this.state === 'open') {
      if (now < this.openUntil) return false;
      this.state = 'half-open';
      this.probe = false;
      log('BREAKER', this.name + ': cooldown elapsed -> HALF-OPEN');
    }
    if (this.state === 'half-open') {
      if (this.probe) return false;
      this.probe = true;
    }
    return true;
  }
  onSuccess() {
    if (this.state === 'open') return; // stale in-flight call must not close it
    this.failures = 0;
    this.probe = false;
    if (this.state === 'half-open') {
      this.state = 'closed';
      log('BREAKER', this.name + ': probe ok -> CLOSED');
    }
  }
  onFailure() {
    if (this.state === 'open') return;
    this.probe = false;
    if (this.state === 'half-open') {
      this.state = 'open';
      this.openUntil = performance.now() + this.cooldownMs;
      log('BREAKER', this.name + ': probe failed -> OPEN ' + this.cooldownMs + 'ms');
      return;
    }
    this.failures++;
    if (this.failures >= this.threshold) {
      this.state = 'open';
      this.openUntil = performance.now() + this.cooldownMs;
      log('BREAKER', this.name + ': ' + this.failures + ' consecutive fails -> OPEN ' + this.cooldownMs + 'ms');
    }
  }
  async call(fn) {
    if (!this.allow()) {
      const e = new Error('fast-fail: ' + this.name + ' is open');
      e.code = 'ECIRCUIT';
      throw e;
    }
    try {
      const value = await fn();
      this.onSuccess();
      return value;
    } catch (e) {
      this.onFailure();
      throw e;
    }
  }
}

// ---------- helpers ----------
function withTimeout(promise, ms, label) {
  let timer;
  const timeout = new Promise(function (_, reject) {
    timer = setTimeout(function () {
      const e = new Error('timeout after ' + ms + 'ms (' + label + ')');
      e.code = 'ETIMEOUT';
      reject(e);
    }, ms);
  });
  return Promise.race([promise, timeout]).then(
    function (v) { clearTimeout(timer); return v; },
    function (e) { clearTimeout(timer); throw e; }
  );
}

function fib(n) { return n < 2 ? n : fib(n - 1) + fib(n - 2); }

async function mapLimit(items, limit, fn) {
  const out = new Array(items.length);
  let cursor = 0;
  async function run() {
    while (true) {
      const i = cursor++;
      if (i >= items.length) return;
      out[i] = await fn(items[i], i);
    }
  }
  const runners = [];
  for (let i = 0; i < Math.min(limit, items.length); i++) runners.push(run());
  await Promise.all(runners);
  return out;
}

// ---------- network layer ----------
const net = new TokenBucket(4, 4);
const netStats = { requests: 0, responses: 0, bytes: 0 };

async function fetchJSON(url) {
  await net.take();
  netStats.requests++;
  let res;
  try {
    res = await fetch(url);
  } catch (e) {
    const err = new Error('network error: ' + e.message);
    err.transient = true;
    throw err;
  }
  const text = await res.text();
  netStats.bytes += text.length;
  netStats.responses++;
  if (!res.ok) {
    const err = new Error('HTTP ' + res.status + ' from ' + url);
    err.transient = true;
    throw err;
  }
  try {
    return JSON.parse(text);
  } catch (e) {
    const err = new Error('non-JSON response from ' + url);
    err.transient = true;
    throw err;
  }
}

const breaker = new CircuitBreaker('flaky-api', 5, 400);
let remoteCalls = 0;
let remoteSuccesses = 0;

async function callFlakyApi(payload) {
  return breaker.call(async function () {
    remoteCalls++;
    await sleep(50 + Math.random() * 120);
    // forced failure burst at the start, then ~30% background failures,
    // so the breaker definitely trips and later recovers.
    if (remoteCalls <= 6 || Math.random() < 0.3) {
      const e = new Error('upstream 503');
      e.transient = true;
      throw e;
    }
    remoteSuccesses++;
    return { echo: payload, calls: remoteCalls };
  });
}

// ---------- task definitions ----------
const tasks = [];

function task(name, priority, fn, opts) {
  return Object.assign({
    name: name,
    priority: priority,
    fn: fn,
    maxAttempts: 2,
    timeoutMs: 4000
  }, opts || {});
}

function todoTask(id, priority, maxAttempts) {
  return task('todo#' + id, priority, async function () {
    const todo = await fetchJSON('https://jsonplaceholder.typicode.com/todos/' + id);
    if (!todo || todo.id !== id || typeof todo.title !== 'string') {
      const e = new Error('unexpected todo payload for id ' + id);
      e.transient = false;
      throw e;
    }
    return 'userId=' + todo.userId + ' done=' + todo.completed + ' "' + trim(todo.title, 24) + '"';
  }, { maxAttempts: maxAttempts, timeoutMs: 4500 });
}

function flakyTask(n, priority) {
  return task('flaky-call#' + n, priority, async function () {
    const out = await callFlakyApi({ id: n });
    return 'echo=' + out.echo.id + ' remoteCall#' + out.calls;
  }, { maxAttempts: 6, timeoutMs: 2500 });
}

// deliberately mixed enqueue order: low-priority backlog first, high-priority
// work arrives later and must still be dispatched first.
tasks.push(task('cpu-fib27', 2, async function () {
  const v = fib(27);
  return 'fib(27)=' + v + ' (sync block!)';
}, { maxAttempts: 1, timeoutMs: 2000 }));

tasks.push(todoTask(5, 1, 3));

tasks.push(task('slow-api-1s', 2, async function () {
  const data = await fetchJSON('https://httpbin.org/delay/1.0');
  return 'httpbin ok origin=' + trim(data.origin || '?', 20);
}, { maxAttempts: 2, timeoutMs: 6000 }));

tasks.push(task('timeout-victim', 1, async function () {
  await sleep(900);
  return 'you should never see this';
}, { maxAttempts: 2, timeoutMs: 250 }));

tasks.push(flakyTask(1, 2));
tasks.push(todoTask(1, 0, 3));

tasks.push(task('cpu-fib26', 2, async function () {
  const v = fib(26);
  return 'fib(26)=' + v + ' (sync block!)';
}, { maxAttempts: 1, timeoutMs: 2000 }));

tasks.push(task('poison-400', 1, async function () {
  await sleep(20);
  const e = new Error('schema mismatch: missing field "id"');
  e.transient = false;
  throw e;
}, { maxAttempts: 3, timeoutMs: 1000 }));

tasks.push(todoTask(2, 1, 3));
tasks.push(flakyTask(2, 1));

tasks.push(task('fanout-todos', 0, async function () {
  const ids = [6, 7, 8];
  const rows = await mapLimit(ids, 2, async function (id) {
    const todo = await fetchJSON('https://jsonplaceholder.typicode.com/todos/' + id);
    return id + ':' + todo.completed;
  });
  return 'nested mapLimit(2) batch [' + rows.join(' ') + ']';
}, { maxAttempts: 2, timeoutMs: 6000 }));

tasks.push(todoTask(3, 2, 3));
tasks.push(flakyTask(3, 1));
tasks.push(todoTask(4, 0, 3));
tasks.push(flakyTask(4, 2));

tasks.push(task('doomed-503', 1, async function () {
  await sleep(25);
  const e = new Error('upstream 503 (permanent)');
  e.transient = true;
  throw e;
}, { maxAttempts: 3, timeoutMs: 1000 }));

tasks.push(task('cpu-fib28', 2, async function () {
  const v = fib(28);
  return 'fib(28)=' + v + ' (sync block!)';
}, { maxAttempts: 1, timeoutMs: 2000 }));

// ---------- queue + workers ----------
const queue = new AgingPriorityQueue(0.75);
for (const t of tasks) queue.push(t);

const initialBacklog = queue.size;
const depthSamples = [queue.size];

const stats = {
  dispatched: 0,
  completed: 0,
  failed: 0,
  retries: 0,
  timeouts: 0,
  recovered: 0,
  byPriority: {},
  dlq: []
};
const records = [];
const workerLoad = { w1: 0, w2: 0, w3: 0 };
let active = 0;
let maxActive = 0;
let inflight = 0;

async function runWithRetry(t, wid) {
  let attempt = 0;
  while (true) {
    attempt++;
    try {
      const value = await withTimeout(
        Promise.resolve().then(function () { return t.fn(attempt); }),
        t.timeoutMs,
        t.name
      );
      if (attempt > 1) stats.recovered++;
      return { attempt: attempt, value: value };
    } catch (e) {
      if (e.code === 'ETIMEOUT') stats.timeouts++;
      const retryable = e.transient === true || e.code === 'ETIMEOUT' || e.code === 'ECIRCUIT';
      if (!retryable || attempt >= t.maxAttempts) {
        e.attempts = attempt;
        throw e;
      }
      const base = 60 * Math.pow(2, attempt - 1);
      const backoff = Math.round(base + Math.random() * base * 0.6);
      stats.retries++;
      log('RETRY', '[' + wid + '] ' + t.name + ' try ' + attempt + '/' + t.maxAttempts + ' ' + (e.code || 'ERR') + ', backoff ' + backoff + 'ms');
      await sleep(backoff);
    }
  }
}

async function handleTask(t, wid) {
  stats.dispatched++;
  workerLoad[wid]++;
  const start = performance.now();
  log('DISPATCH', '[' + wid + '] ' + t.name + ' p=' + t.priority + ' aged=' + t.key.toFixed(2) + ' qdepth=' + queue.size);
  try {
    const out = await runWithRetry(t, wid);
    stats.completed++;
    stats.byPriority[t.priority] = (stats.byPriority[t.priority] || 0) + 1;
    const dur = Math.round(performance.now() - start);
    records.push({ name: t.name, start: start, end: performance.now(), ok: true, worker: wid });
    log('OK', '[' + wid + '] ' + t.name + ' ' + dur + 'ms attempt=' + out.attempt + ' -> ' + trim(out.value, 48));
  } catch (e) {
    stats.failed++;
    stats.dlq.push({ task: t.name, error: e.code || e.name, detail: trim(e.message, 90), attempts: e.attempts || 1 });
    records.push({ name: t.name, start: start, end: performance.now(), ok: false, worker: wid });
    log('DLQ', '[' + wid + '] ' + t.name + ' -> dead letter (' + (e.code || e.name) + ') after ' + (e.attempts || 1) + ' attempt(s)');
  }
}

async function worker(wid) {
  while (true) {
    if (queue.size === 0) {
      if (inflight === 0) return;
      await sleep(10);
      continue;
    }
    const t = queue.pop();
    inflight++;
    active++;
    if (active > maxActive) maxActive = active;
    try {
      await handleTask(t, wid);
    } finally {
      active--;
      inflight--;
    }
  }
}

const sampler = setInterval(function () { depthSamples.push(queue.size); }, 150);

log('QUEUE', initialBacklog + ' tasks enqueued across priorities 0/1/2; starting 3 workers');
await Promise.all([worker('w1'), worker('w2'), worker('w3')]);
clearInterval(sampler);
depthSamples.push(queue.size);

// ---------- reporting ----------
function sparkline(values) {
  if (values.length === 0) return '(no samples)';
  const chars = '.:-=+*#@';
  const max = Math.max.apply(null, values.concat([1]));
  let out = '';
  for (const v of values) {
    out += chars[Math.min(chars.length - 1, Math.floor((v / max) * (chars.length - 1)))];
  }
  return out;
}

const elapsed = rel();
console.log('');
console.log('=== QUEUE DEPTH (sampled every 150ms, 0=' + initialBacklog + ' backlog) ===');
console.log(sparkline(depthSamples));
console.log('peak queued=' + initialBacklog + '  max in-flight=' + maxActive + '  final depth=' + queue.size);

console.log('');
console.log('=== TIMELINE (one char = 150ms; # ok, x dead-letter) ===');
const ordered = records.slice().sort(function (a, b) { return a.start - b.start; });
for (const r of ordered) {
  const offset = Math.max(0, Math.round(r.start / 150));
  const len = Math.max(1, Math.round((r.end - r.start) / 150));
  const label = ('[' + r.worker + '] ' + trim(r.name, 18)).padEnd(24, ' ');
  console.log(label + '|' + ' '.repeat(offset) + (r.ok ? '#' : 'x').repeat(len));
}

console.log('');
console.log('=== SUMMARY ===');
const summary = {
  config: {
    workers: 3,
    queue: 'aging priority (0=high, 2=low)',
    agingPerSec: 0.75,
    rateLimiter: '4-request burst, 4 req/s',
    breaker: '5 consecutive fails -> open 400ms',
    retry: 'exponential backoff with jitter'
  },
  totals: {
    enqueued: tasks.length,
    dispatched: stats.dispatched,
    completed: stats.completed,
    deadLettered: stats.failed,
    retries: stats.retries,
    timeoutEvents: stats.timeouts,
    recoveredAfterRetry: stats.recovered
  },
  completedByPriority: stats.byPriority,
  workerLoad: workerLoad,
  network: {
    requests: netStats.requests,
    responses: netStats.responses,
    bytes: netStats.bytes,
    rateLimiterWaits: net.starved
  },
  breaker: {
    state: breaker.state,
    consecutiveFailures: breaker.failures,
    simulatedRemoteCalls: remoteCalls,
    simulatedRemoteSuccesses: remoteSuccesses
  },
  deadLetterQueue: stats.dlq,
  elapsedMs: elapsed,
  throughputPerSec: Number((stats.completed / (elapsed / 1000)).toFixed(2)),
  eventLines: events.length
};
console.log(JSON.stringify(summary, null, 2));
console.log('');
console.log('DRAINED in ' + elapsed + 'ms. hierarchy: aging-priority queue -> 3 workers -> token bucket -> breaker/retry -> timeout race.');
