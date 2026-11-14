# nested-function-declaration-settimeout.js

Node 24 baseline (`v24.22.2`):

```text
start
sync end
timeout
other timer
```

Current go_js behavior:

```text
stdout:
start
sync end

stderr:
runtime error: TypeError: The callback argument must be of type function

exit: 1
```

**Failure.** `onTimeout` is a function declaration created inside the IIFE. Node accepts it as a timer callback; go_js does not. The same function works when it is declared at top level or when it is a nested function expression. The problem is the nested declaration value, not the timer or the delay.

**Not covered.** This case does not assert precise delay ordering or `clearTimeout`. A nested declaration passed to `setInterval` fails with the same `TypeError` in a manual check, but that variant is not included here.
