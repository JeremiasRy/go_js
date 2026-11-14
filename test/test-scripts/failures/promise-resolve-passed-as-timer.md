# promise-resolve-passed-as-timer.js

Node 24 baseline (`v24.22.2`):

```text
start
sync end
then undefined
other timer
```

Current go_js behavior:

```text
stdout:
start
sync end
other timer

stderr:
(empty)

exit: 0
```

**Failure.** `setTimeout(resolve, 10)` should settle the promise with `undefined` and queue the `.then` callback. go_js runs the timer, and the later 40 ms timer still fires, but the `.then` callback never runs and no error is reported. The arrow-executor version also fails, so this is not the executor-parameter capture bug from `promise-resolver-timer-await.js`.

**Not covered.** The value-carrying arrow path (`setTimeout(() => resolve("x"), ...)`) works when the executor is also an arrow in a manual check; it is not part of this repro. This case also does not test rejection or whether the promise is settled internally.
