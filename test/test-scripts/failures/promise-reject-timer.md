# promise-reject-timer.js

Node 24 baseline (`v24.22.2`):

```text
timer
caught rejected
other timer
```

Current go_js behavior:

```text
stdout:
timer

stderr:
runtime error: ReferenceError: Cannot access variable before initialization

exit: 1
```

**Failure.** This is the rejection counterpart of `promise-resolver-timer-await.js`. The non-arrow executor's `reject` parameter is referenced from a nested timer callback, and go_js throws a TDZ `ReferenceError` instead of rejecting. The `.catch` handler never runs. The all-arrow reject equivalent works in a manual check.

**Not covered.** Unhandled-rejection behavior is not tested because this repro catches the rejection. It also does not assert the rejection value beyond its message or the exact timer delay.
