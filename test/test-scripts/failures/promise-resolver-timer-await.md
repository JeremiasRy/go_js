# promise-resolver-timer-await.js

Node 24 baseline (`v24.22.2`):

```text
async start
sync end
timer
async resumed resolved
other timer
```

Current go_js behavior:

```text
stdout:
async start
sync end
timer

stderr:
runtime error: ReferenceError: Cannot access variable before initialization

exit: 1
```

**Failure.** The Promise executor is a non-arrow function expression and its `resolve` parameter is referenced from the nested timer callback. go_js throws a TDZ `ReferenceError` when the timer runs, so the `await` never resumes and `async resumed` is never printed. The same flow with an arrow executor works in a manual check, so async/await and timer dispatch are not broken by themselves; capturing the executor's resolver parameter in a nested callback is.

**Not covered.** The rejection path is covered separately by `promise-reject-timer.js`. This case does not test delay accuracy, timer ordering beyond the two markers, or non-string resolution values.
