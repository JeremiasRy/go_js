# promise-resolver-nested-sync.js

Node 24 baseline (`v24.22.2`):

```text
start
executor
nested call
after resolve
executor end
sync end
then resolved
other timer
```

Current go_js behavior:

```text
stdout:
start
executor
nested call
sync end
other timer

stderr:
(empty)

exit: 0
```

**Failure.** Calling `resolve` from a nested synchronous function silently ends the executor at that point. `after resolve` and `executor end` are skipped, and the `.then` handler never runs. The process still exits successfully with no diagnostic. A direct `resolve("resolved")` call in the executor body works in a manual check, so the failure is specific to resolving through the nested function call.

**Not covered.** The repro cannot observe whether the promise is settled internally. It does not test rejection or an asynchronous gap; those are covered by the other resolver cases.
