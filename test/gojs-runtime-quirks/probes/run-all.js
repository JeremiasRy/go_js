// Runs every numbered probe with the same Node binary that runs this file.
//
// Usage:
//   node probes/run-all.js
//   npx -y node@24 probes/run-all.js
//
// Each probe is expected to exit 0 under Node 24. Network probes depend on
// httpbin.org, jsonplaceholder.typicode.com and example.com.

import { spawnSync } from 'node:child_process';
import { readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const files = readdirSync(here)
  .filter((name) => /^\d{2}-.*\.js$/.test(name))
  .sort();

let failed = 0;

for (const file of files) {
  console.log(`\n${'='.repeat(72)}\n${file}\n${'='.repeat(72)}`);
  const result = spawnSync(process.execPath, [join(here, file)], {
    stdio: 'inherit',
    timeout: 20000
  });

  if (result.error) {
    failed++;
    console.error(`ERROR: ${file}: ${result.error.message}`);
    continue;
  }
  if (result.status !== 0) {
    failed++;
    console.error(`FAILED: ${file} (status ${result.status}, signal ${result.signal ?? 'none'})`);
  }
}

console.log(`\n${'='.repeat(72)}`);
console.log(`${files.length - failed}/${files.length} probes exited 0 under ${process.version}`);
if (failed > 0) process.exitCode = 1;
