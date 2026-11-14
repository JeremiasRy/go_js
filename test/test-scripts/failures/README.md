# Async and timer failure repros

Manual repros for the `missing-async-features` work. The reference implementation is Node v24.22.2 (`node:24-slim`).

Each repro added in this batch has a same-named `.md` with the expected Node 24 stdout, the current go_js stdout/stderr, and the boundary of the failure.

These files are not discovered by the Go test runner: `test/cases.go` only reads top-level `.js` files in `test/test-scripts`. They stay here until the behavior is fixed and the case can move into the passing suite.

## Run a case

```sh
docker run --rm -v "$PWD:/work" -w /work node:24-slim \
  node test/test-scripts/failures/<case>.js

(cd runtime && go build -o /tmp/go_js .)
/tmp/go_js --source-type=script test/test-scripts/failures/<case>.js
```

## Cases

| File | Node 24 | Current go_js |
| --- | --- | --- |
| `nested-function-declaration-settimeout.js` | Timer callback runs | `TypeError`, exit 1 |
| `nested-function-declaration-queuemicrotask.js` | Microtask runs | Process hangs |
| `promise-resolver-timer-await.js` | `await` resumes | `ReferenceError`, exit 1 |
| `promise-reject-timer.js` | `.catch` runs | `ReferenceError`, exit 1 |
| `promise-resolver-nested-sync.js` | Executor and `.then` complete | Executor truncated, `.then` skipped, exit 0 |
| `promise-resolve-passed-as-timer.js` | `.then undefined` runs | `.then` skipped, exit 0 |

## Not covered

`fetch` and timer interleaving are not included. In the MCP run that prompted these repros, the Go-backed fetch kept the event loop busy for the duration of the request, so timer callbacks did not run while a slow request was in flight. A reliable regression test needs a controlled slow HTTP endpoint rather than a public URL.
