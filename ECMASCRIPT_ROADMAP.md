# ECMAScript implementation roadmap

## Goal and priorities

Expand GO_JS into a more useful, predictable ECMAScript runtime by completing core semantics, adding everyday language and library features, and then extending asynchronous execution, classes, and modules.

This is the runtime implementation plan. All named JavaScript test specifications below have been written ahead of the corresponding runtime features: 129 positive entrypoints, seven negative entrypoints, and eight module fixtures, with six additional failure fixtures for imported-module scenarios. Positive tests are in `test/test-scripts/`; expected-error programs are in `test/negative-scripts/`.

| Milestone | Priority | Outcome | Dependencies |
| --- | --- | --- | --- |
| M0 — Test infrastructure | First | Reliable failures, execution modes, and diagnostics | Existing differential runner |
| M1 — Semantic foundations | Highest | Correct evaluation, bindings, operators, and completion handling | M0 |
| M2 — Functions, objects, and iteration | High | Reusable call, property, and iterable protocols | M1 |
| M3 — Everyday standard library | High | Useful Array, String, Object, Number, Math, JSON, and collection APIs | Relevant M1/M2 primitives |
| M4 — Promises and generators | High | Composable asynchronous execution and resumable functions | M1 completion handling; M2 calls/iteration |
| M5 — Classes and modules | Medium | Larger programs with inheritance and isolated, live module bindings | M2; M4 for asynchronous modules |
| M6 — Broader compatibility | Later | BigInt, RegExp, binary data, reflection, and legacy bindings | Relevant earlier primitives |

Dependencies apply per feature: for example, numeric library work can begin after coercion is correct, while `Array.from` needs the iterable protocol. M4's promise work can proceed alongside M3 once its foundations are ready.

### Run the test suite

```sh
./test.sh
```

The existing Go/Docker runner discovers all top-level JavaScript tests and reads source-goal overrides and negative-test expectations from `test/cases.json`. It runs every case against Node and GO_JS, compares stdout, and checks expected error classes and phases. Three environment-sensitive files run in both Script and Module goals, giving 209 regression cases including the original 70. Each process has a timeout, and the suite exits nonzero on failures after reporting all cases, the AoC puzzle, and benchmarks. `test/node-reference.js` is the runner's single-case Node adapter for source goals and diagnostics.

## Current implementation baseline

The initial review covered the 70 top-level JavaScript regression scripts present before the roadmap additions and the associated compiler, VM, and native implementations.

| Area | Existing examples | Expansion needed |
| --- | --- | --- |
| Expressions and control flow | `arithmetic.js`, `bitwise.js`, `logical-shortcircuit.js`, `optional-chain.js`, `switch-case-numeric.js` | Evaluation order, additional operators, `default`, labels, `do...while`, and `finally` |
| Functions and bindings | `function-parameters.js`, `closure.js`, `arguments.js`, `rest-parameter.js`, `block-variables.js` | Default/pattern parameters, TDZ, immutable bindings, per-iteration environments, and explicit call/bind APIs |
| Objects and arrays | `Object-methods.js`, `object-destructuring-rest.js`, `array-map.js`, `array-reduce.js` | Property descriptors, complete key ordering, recursive patterns, sparse-array semantics, and additional methods |
| Strings | `string-split.js`, `string-replase.js`, `string-starts-with.js`, `template-literal.js` | UTF-16 semantics, broader methods, replacement rules, and tagged templates |
| Collections and iteration | `map-object.js`, `set-object.js`, `array-for-of-loop.js` | Symbols, user-defined iterators, iterator closing, collection constructors/methods, and key equality |
| Async and generators | `async-fn-1.js`, `async-fn-2.js`, `micro-macro-task-queue.js`, `generator.js` | Promise chaining/rejection, thenable assimilation, combinators, resumable generator operations, and async iteration |
| Classes and modules | `class-1.js`, `class-2.js`, `object-prototype.js`, `import-statement.js` | Inheritance, per-instance field evaluation, static/private members, import forms, live bindings, and cycles |

Important implementation details behind these gaps:

- `runtime/compiler/compiler.go` emits both operands of logical expressions and treats optional chains as ordinary member expressions. Its unary operator map currently contains only negation and logical NOT.
- `runtime/compiler/prepass.go` registers ordinary variable declarations as `LET` and visits only the try block and catch handler of a try statement.
- `runtime/native/object.go` stores string-keyed values without descriptors; the internal prototype link uses a member named `prototype`. The object model needs a distinct internal `[[Prototype]]` slot and an ordinary constructor `.prototype` property.
- `runtime/object/iterator.go` uses Go-side slices of keys/values. This provides the current built-in iteration path, but user-defined ECMAScript iterators need observable protocol calls.
- `runtime/native/prototype.go` registers `Promise.resolve` and `.then`; the VM's `.then` path currently returns `undefined`.
- `runtime/native/string.go` uses Go string byte lengths. `runtime/native/date.go` currently provides `Date.now`.
- The README deliberately omits `var`; its proposed expansion is in M6.

### Confirmed semantic gaps

Six isolated probes were run against a freshly built GO_JS and local Node v20.20.2. These provide the initial regression targets; the Docker test environment uses Node 24.

| Probe | Node result | Current GO_JS result | Planned regression |
| --- | --- | --- | --- |
| RHS call counter after `false && effect()`, `true || effect()`, `0 ?? effect()` | `0` | `3` | `logical-shortcircuit-side-effects.js`, `nullish-shortcircuit.js` |
| `!-1` | `false` | `true` | `truthiness-coercion.js` |
| Reassign a `const` inside `try/catch` | Assignment is caught | Value changes to `2` | `const-assignment.js` |
| Trace a `try/catch/finally` with no exception | `try`, then `finally` | Only `try` | `try-finally.js` |
| Read `undefined.x` inside `try/catch` | Access is caught | Produces `undefined` | `nullish-property-access.js` |
| `"😀".length` | `2` | `4` | `string-utf16.js` |

## M0 — Make the tests reliable implementation gates

Implementation areas: `test/main.go`, `Dockerfile.test`, `runtime/main.go`, `runtime/flags/flags.go`, and compiler diagnostics.

1. **Accurate discovery and failures — integrated.** The Docker runner selects `.js` entrypoints, counts actual cases, applies per-process timeouts, and exits nonzero on unexpected failures or output mismatches.
2. **Explicit source goals — integrated.** `test/cases.json` configures Script/Module execution. The Node adapter evaluates those goals explicitly, and GO_JS accepts `--source-type`. Pin the Node reference version used by CI as a follow-up.
3. **Expected failures — integrated.** Metadata supplies the expected error class and phase (parse, module linking, or execution). GO_JS's `--json-errors` diagnostics distinguish parser rejections from compiler/VM errors; crashes and timeouts fail the case. Standalone invalid programs live in `test/negative-scripts/` because a surrounding `try/catch` cannot catch a syntax error in its own file.
4. **Unsupported-feature diagnostics.** Reject unhandled AST nodes and operators with a source location. The current compiler's unchecked operator lookups and missing default branches can turn unsupported syntax into incorrect bytecode or omitted behavior.
5. **Execution-mode coverage.** Expose the existing `flags.EnableJIT` setting through a testable runtime option. Compare relevant cases with JIT enabled and disabled; newly introduced bytecodes must have a correct interpreter fallback.

Initial negative fixtures:

| Path | Expected behavior |
| --- | --- |
| `test/negative-scripts/duplicate-lexical-binding.js` | Parse-time `SyntaxError` for duplicate lexical declarations |
| `test/negative-scripts/const-without-initializer.js` | Parse-time `SyntaxError` for an ordinary `const` declaration without an initializer |
| `test/negative-scripts/invalid-assignment-target.js` | Parse-time `SyntaxError` for an invalid assignment target |
| `test/negative-scripts/uncaught-type-error.js` | Execution-time `TypeError`, reported as an expected runtime failure |

Directory discovery is flat. Module fixtures are loaded through imports, and negative entrypoints are selected explicitly in `test/cases.json`.

## M1 — Complete semantic foundations

Implementation areas: `runtime/compiler/prepass.go`, `runtime/compiler/compiler.go`, `runtime/chunk/chunk.go`, `runtime/vm/vm.go`, `runtime/value/value.go`, and native string/error support.

Build shared conversion, equality, reference, binding, and completion operations so later features use the same rules.

| Feature | `.js` files | Required cases |
| --- | --- | --- |
| Truthiness, coercion, and equality | `truthiness-coercion.js`, `equality-edge-cases.js` | Negative numbers are truthy; `0`, `-0`, and NaN are falsy; whitespace/numeric-string conversion; null/undefined distinctions; object identity; NaN and signed-zero equality. Establish shared `ToBoolean`, `ToNumber`, `ToString`, `ToPrimitive`, and equality operations. |
| Standard runtime errors | `error-types.js`, `nullish-property-access.js` | Catchable `TypeError`, `ReferenceError`, `RangeError`, and `SyntaxError`; stable `name`/`message`; invalid calls and ordinary null/undefined member access. Runtime errors must participate in JS exception handling. |
| Lazy logical evaluation | `logical-shortcircuit-side-effects.js`, `nullish-shortcircuit.js` | Skipped RHS calls, assignments, and throws; operand-valued results; `0`, `false`, and `""` preserved by `??`; left operand evaluated once. |
| Optional property access and calls (ES2020) | `optional-chain-side-effects.js`, `optional-call.js` | `?.prop`, `?.[expr]`, `?.()`; skipped keys/arguments; method receiver preserved; chain grouping boundaries; non-callable present values still throw. |
| Lexical bindings and hoisting | `const-assignment.js`, `temporal-dead-zone.js`, `loop-closure-bindings.js`, `function-hoisting.js` | Immutable binding versus mutable object contents; reads/writes before initialization; shadowing; `typeof` in the TDZ; closures capture distinct loop iterations; declaration hoisting in the appropriate scope. |
| Global environments and strictness | `global-environment.js`, `strict-mode.js` | Explicit script/module goals; globalThis and standard global constants; global lexical bindings versus object properties; directive strictness and implicit module/class strictness; unbound-call this behavior and unresolved assignments. Run source-goal-sensitive cases in both modes through metadata. |
| Missing unary/binary operators | `unary-operators.js`, `loose-not-equals.js`, `bitwise-shifts.js`, `sequence-expression.js` | `typeof`, `void`, unary `+`, `~`, `!=`, `<<`, `>>`, `>>>`; `typeof` on an unresolved name; 32-bit conversion and masked shift counts; comma-expression order and final value. |
| Assignment and update expressions | `compound-assignment.js`, `logical-assignment.js`, `member-update.js` | Arithmetic/bitwise compound assignments; `&&=`, `||=`, `??=` (ES2021); prefix/postfix result values; chained assignment values; computed base/key evaluated once; RHS skipped when appropriate. Extend accessor-sensitive cases after M2. |
| Completion and cleanup | `try-finally.js`, `finally-abrupt-completion.js` | `try/finally` and `try/catch/finally`; nested handlers; return/throw/break/continue cross finalizers; a finalizer's abrupt completion overrides the pending completion; restoration of stack/frame state. |
| Additional control flow | `do-while-loop.js`, `labeled-loops.js`, `switch-default.js` | Body runs before the first `do...while` test; nested labeled break/continue; switch default placement and fallthrough; discriminant evaluated once; switch inside loops preserves the correct break/continue target. |
| UTF-16 string foundation | `string-utf16.js`, `template-escapes.js` | Code-unit length/indexing and `charAt`/`charCodeAt`; astral characters, combining characters, lone surrogates, and empty strings; ordinary template literals use cooked escape sequences. |

Completion handling should carry the pending kind/value/target explicitly. The same mechanism will support iterator closing, generator cleanup, and asynchronous exception propagation.

## M2 — Functions, property semantics, and iterable protocols

Implementation areas: compiler binding/call generation, `runtime/object/function.go`, `runtime/object/object.go`, `runtime/object/iterator.go`, `runtime/native/object.go`, `runtime/native/prototype.go`, `runtime/native/commonHandles.go`, the VM, and heap tracing.

Implement the property model before symbol-keyed operations; implement the iterator protocol before completing iterable consumers.

| Feature | `.js` files | Required cases |
| --- | --- | --- |
| Default and destructured parameters (ES2015) | `function-default-parameters.js`, `function-destructured-parameters.js` | Defaults trigger only for missing/undefined arguments; earlier parameters are visible; later parameters have TDZ behavior; defaults run per call; nested patterns and rest; parameter/body scope interactions and function length. |
| Recursive destructuring | `destructuring-nested-defaults.js`, `destructuring-assignment.js`, `array-destructuring-rest.js` | Nested object/array patterns; aliases, elisions, defaults, rest; assignment to existing bindings/members; computed keys; undefined versus null; evaluate sources once. Array patterns consume iterators and close them when required. |
| Function invocation and construction | `function-call-apply-bind.js`, `arrow-this-arguments.js`, `constructor-function.js` | `.call`, array-like `.apply`, `.bind` with partial arguments; lexical arrow `this`/`arguments`; detached methods; ordinary constructor functions, returned objects, `new.target`, bound construction, and non-constructible arrows. |
| Object literal keys and spread | `object-computed-properties.js`, `object-spread-expressions.js` | String/numeric/computed keys; shorthand methods; spread arbitrary expressions; left-to-right evaluation and overwrite order; own enumerable properties, getter evaluation, and symbol keys. |
| Descriptors and accessors | `object-property-descriptors.js`, `object-accessors.js` | `defineProperty`, `defineProperties`, descriptor inspection; data/accessor distinction; writable/enumerable/configurable flags; inherited accessors with the original receiver; getter/setter exceptions; rejected descriptor redefinitions. |
| Prototypes, deletion, and enumeration | `object-prototype-chain.js`, `delete-operator.js`, `object-enumeration-order.js`, `instanceof-operator.js` | Internal prototype independent of a property named `prototype`; null prototypes; `getPrototypeOf`/`setPrototypeOf`; deletion results and strict-mode errors; own integer-index keys before insertion-ordered strings; inherited `for...in` with duplicate suppression; ordinary `instanceof`. |
| Object integrity | `object-integrity.js` | `preventExtensions`, `seal`, `freeze`, and matching inspection APIs; strict/sloppy writes where applicable; shallow freezing; array length/property interactions. |
| Symbol primitives (ES2015) | `symbol-values.js`, `symbol-properties.js`, `symbol-to-primitive.js` | Unique identities, `Symbol.for`/`keyFor`, `typeof`; symbol property keys and `getOwnPropertySymbols`; symbol exclusion from string-key enumeration; `Symbol.toPrimitive` hints and error behavior; `Symbol.hasInstance` extends the instanceof cases. |
| Iterable/iterator protocol (ES2015) | `custom-iterator.js`, `iterator-close.js`, `iterable-spread-destructuring.js` | `Symbol.iterator`, `next`, result-object validation, done/value ordering; arrays and strings as iterables; string iteration by code point; spread into arrays/calls; early destructuring completion and loop break/return/body-throw cleanup; completion precedence when `return()` throws. |
| Tagged templates (ES2015) | `tagged-template.js` | Tag receiver and substitution order; cooked/raw strings; template-object identity per source site; frozen template arrays; invalid cooked escapes in tagged literals. |

All heap-held references introduced here need tracing: accessor functions, symbol keys, bound targets/arguments, prototype links, and iterator state.

## M3 — Add everyday standard-library features

Implementation areas: existing native array/string/object/map/set/date implementations; new number/math/JSON implementations; global registration in the compiler; native call dispatch in the VM.

| Feature | `.js` files | Required cases |
| --- | --- | --- |
| Array structure and existing callbacks | `array-sparse-length.js`, `array-callback-semantics.js` | Holes versus explicit undefined; out-of-range reads; array-index versus ordinary property keys; length growth/truncation; map/filter/forEach/reduce callback arguments, thisArg where supported, initial length snapshot, mutation during traversal, and empty reduce without an initial value. |
| Array search/predicates | `array-search.js`, `array-predicates.js` | `indexOf`, `lastIndexOf`, `includes`, `find`, `findIndex`, `some`, `every`, and `at`; negative positions; early exit; NaN equality differences; method-specific hole handling. |
| Array mutation and copying | `array-pop-unshift.js`, `array-slice-splice.js`, `array-concat-sort.js` | Return values and mutation boundaries; negative/clamped indices; sparse arrays; concat spreading rules; stable sort, default string ordering, comparator callbacks and exceptions. |
| Array factories and newer methods | `array-factories.js`, `array-flat-flatmap.js`, `array-copying-methods.js` | `Array.isArray`, `of`, iterable/array-like `from`; mapping and iterator cleanup; flat depth and holes; ES2023 `toSorted`, `toReversed`, `toSpliced`, `with`, `findLast`, and `findLastIndex`; verify copy versus mutation behavior. |
| String slicing/search/normalization | `string-slice-substring.js`, `string-search-trim.js`, `string-pad-repeat.js`, `string-code-points.js` | Slice versus substring index rules; indexOf/lastIndexOf/endsWith and existing search positions; trim variants; lower/upper casing; padding/repeat boundaries and errors; codePointAt/fromCodePoint and Unicode normalization. |
| String replacement | `string-replacement.js`, `string-replace-all.js` | First versus all matches; string-search replacement callbacks; `$$`, `$&`, prefix/suffix substitutions; empty search strings; split separator/limit rules. Add regex-specific overload cases with M6. |
| Global conversions and numeric utilities | `global-conversions.js`, `number-methods.js`, `number-parsing.js`, `math-methods.js` | `Number`, `String`, `Boolean` calls and wrappers; NaN/Infinity; number predicates/constants and formatting; parseFloat and parseInt whitespace/sign/radix/prefix/trailing-junk behavior; deterministic Math methods, signed zero, infinities, and NaN. |
| Object utilities | `object-entries-from-entries.js`, `object-assign-has-own.js`, `object-is.js` | entries/fromEntries round trips and iterator closing; assign source order and getters; enumerable symbol copying; `Object.hasOwn` on null-prototype/shadowing objects; SameValue behavior for NaN and signed zero. |
| Complete Map and Set | `map-methods-iteration.js`, `set-methods-iteration.js`, `collection-key-equality.js` | Iterable constructors; chainable set/add; delete/clear; keys/values/entries/forEach; read-only size accessors; insertion order and mutation during iteration; SameValueZero, equal string values, and object identity. |
| JSON | `json-parse.js`, `json-stringify.js` | Nested values and escaping; invalid JSON; reviver/replacer order and omission; toJSON; spacing; own enumerable key order; undefined/functions in objects versus arrays; nonfinite numbers; cyclic input errors. |
| Useful Date behavior | `date-utc.js`, `date-invalid.js` | Construction from epoch milliseconds and explicit ISO strings; UTC/getTime/getUTC*/setUTC*/toISOString/parse operations; rollover and invalid dates. Use fixed inputs and explicit time zones for reproducible output. |

When a method has additional protocol hooks, include them with the dependency that enables them: for example, concat's `Symbol.isConcatSpreadable`, array species construction, and regex replacement hooks. Track these cases alongside the feature's basic cases.

## M4 — Make async execution and generators composable

Implementation areas: `runtime/native/promise.go`, `runtime/native/asyncFunction.go`, `runtime/native/generator.go`, `runtime/vm/vm.go`, `runtime/eventloop/`, `runtime/queue/`, and `runtime/heap/heap.go`.

Promise state needs a settlement-once rule and multiple queued reactions. Each `.then` produces a new promise. Suspended async/generator invocations need their own execution state, including exception/completion state and GC-visible references.

| Feature | `.js` files | Required cases |
| --- | --- | --- |
| Promise construction and settlement | `promise-settlement.js`, `promise-rejection.js` | Synchronous executor; resolve/reject functions; executor throws; first settlement wins; handlers attached before/after settlement; multiple listeners; `.catch` and rejection recovery. |
| Chaining and resolution procedure | `promise-chaining.js`, `promise-thenables.js` | New promise from each then; omitted handlers propagate values/reasons; returned promises/thenables are assimilated; handler/getter throws; resolve/reject called repeatedly; self-resolution rejection; thenable jobs run in the right order. |
| Promise finalizers (ES2018) | `promise-finally.js` | Preserve original value/reason; await returned thenables; a throw or rejected result replaces the outcome; callback receives no settlement argument. |
| Promise combinators | `promise-all.js`, `promise-race.js`, `promise-all-settled.js`, `promise-any.js` | Iterable inputs and plain values; input-order results; fulfillment/rejection timing; empty-input behavior, including pending `race([])`; `AggregateError.errors` ordering; abrupt iterator handling. |
| Complete async/await | `await-values-errors.js`, `async-concurrent-calls.js`, `promise-job-order.js` | Await primitives and thenables; thrown/rejected values reach catch/finally; async returns always adopt into promises; concurrent invocations retain independent locals; asynchronous resumption and FIFO reaction-job order. |
| Generator lifecycle | `generator-resume.js`, `generator-return-throw.js`, `generator-delegation.js`, `for-of-generators.js` | `.next(value)`, return values, yield without an operand, completed generators, injected throw/return, finally execution, `yield*` forwarding, reentrancy errors, and protocol-compatible iteration. |
| Async iteration (ES2018) | `async-generator.js`, `for-await-of.js` | Async generator next/return/throw promises; queued requests; `Symbol.asyncIterator`; sync-iterator fallback; awaited values; break/error cleanup and rejection propagation. |

Use deterministic promise-job traces. The existing `setTimeout` and `queueMicrotask` tests additionally exercise host scheduling; ECMAScript promise tests should assert observable job order without depending on elapsed milliseconds.

## M5 — Expand classes and modules

Implementation areas: class AST/prepass/compilation, `runtime/native/class.go`, callable/constructor handling, and a module registry with separate loading, linking, and evaluation steps.

| Feature | `.js` files | Required cases |
| --- | --- | --- |
| Class inheritance and expressions (ES2015) | `class-extends-super.js`, `class-expressions.js` | Derived/default constructors; super calls and methods; receiver identity; inherited statics; new.target and instanceof; this-before-super errors; named class-expression scope. |
| Fields and static/private members | `class-field-initialization.js`, `class-static-members.js`, `class-private-fields.js`, `class-static-block.js` | Field initializers run for each instance in source order; fresh object-valued fields; derived-field timing; accessors/computed names; static methods/fields/blocks; private brands, methods/accessors, and `#field in object`. |
| Import/export forms | `module-import-forms.js` | Default, named, aliased, namespace, and side-effect imports; export lists; default exports; re-exports and export-star resolution; module namespace behavior. |
| Module linking and evaluation | `module-live-bindings.js`, `module-cycles.js`, `module-evaluation-once.js` | Separate module scopes; live read-only imports; cached identity/evaluation; cyclic graphs; initialization order and TDZ; exported functions closing over their own module environment. |
| Dynamic import and top-level await | `module-dynamic-import.js`, `module-top-level-await.js` | Import returns a promise; shares the module cache; load/link/evaluation failures reject; dependency evaluation waits for top-level await and propagates rejection. |

Fixtures under `test/test-scripts/test-module/roadmap/`:

- `exports.js` and `reexports.js`: default/named bindings and re-export chains.
- `counter.js`: mutable exported state and an exported mutator for live-binding tests.
- `cycle-a.js` and `cycle-b.js`: a cycle whose successful accesses occur after initialization.
- `side-effect.js`: an observable initialization effect for repeated-import tests.
- `async-dependency.js` and `async-failure.js`: controlled top-level-await fulfillment/rejection.

Separate negative entries cover `test/negative-scripts/module-missing-export.js` (link error), `test/negative-scripts/module-import-assignment.js` (execution error), and `test/negative-scripts/class-private-undeclared.js` (parse error). Negative module dependencies live in `test/negative-scripts/fixtures/`.

## M6 — Broader compatibility after the shared machinery is ready

| Feature | `.js` files | Implementation focus and cases |
| --- | --- | --- |
| RegExp | `regexp-literals.js`, `regexp-exec-test.js`, `regexp-flags-last-index.js`, `regexp-string-methods.js` | Parser literal/flag validation plus a matching backend covering ECMAScript captures, backreferences, lookarounds, Unicode behavior, and UTF-16 indices; constructor/test/exec; g/y lastIndex; named captures; string match/search/replace/split/matchAll integration. |
| BigInt (ES2020) | `bigint-literals.js`, `bigint-arithmetic.js`, `bigint-coercion.js` | Exact arbitrary-precision literals and values; arithmetic, comparisons, bitwise operations, truncating division; typeof; mixed Number/BigInt errors; conversions, asIntN/asUintN, and default JSON serialization errors. |
| Binary data | `array-buffer.js`, `typed-array.js`, `data-view.js` | ArrayBuffer and typed-array constructors; shared views versus copies; byte offsets/lengths; integer wrapping and clamping; bounds/alignment errors; DataView endianness. BigInt views depend on BigInt. |
| Reflect and Proxy (ES2015) | `reflect-operations.js`, `proxy-traps.js`, `proxy-invariants.js`, `proxy-revocable.js` | Reuse property/call/construct internal operations; receiver propagation, descriptors, ownKeys, extensibility, apply/construct, invariant violations, and revocation. |
| `var` compatibility | `var-hoisting.js`, `var-function-scope.js`, `var-global-bindings.js` | Function-scoped hoisting/undefined initialization; redeclaration rules; var versus lexical loop capture; top-level script var properties and their attributes; module-scoped var bindings. |

These subsystems have separate design decisions: RegExp matching semantics, primitive representation for BigInt, binary storage/view ownership, and interceptable object operations. Give each a focused design before implementation.

## Test-writing and completion criteria

- Use small, focused files following the existing lowercase hyphenated naming pattern. Split large feature rows into implementation-sized changes.
- Print primitive values, booleans, counters, or explicit trace strings. For collection order, log entries individually or join known primitive values. This avoids coupling language semantics to console object formatting.
- Include normal, boundary, and error cases. Catch expected execution errors and log their class; give parse/link failures standalone metadata-driven fixtures. Avoid engine-specific stack traces and diagnostic wording.
- Assert evaluation order and counts where observable. A result-only test can pass even when an expression incorrectly runs a skipped branch.
- Add positive entrypoints directly to `test/test-scripts/`; configure source-goal overrides and negative cases in `test/cases.json`. The Docker differential runner executes them together as each implementation milestone lands.
- Run the added cases and existing regressions against the pinned Node reference. For changes to bytecode/calls/value semantics, compare interpreter and JIT-enabled behavior and update opcode names, decoding, and eligibility handling as needed.
- Add focused Go tests where JS output cannot establish an invariant, such as GC reachability of suspended state or a compiler rejection before execution. Exercise relevant features under allocation pressure without relying on nondeterministic collection timing.
- Expand selected [Test262](https://github.com/tc39/test262) coverage as the runner gains source-goal and negative-test support. Use the [ECMAScript specification](https://tc39.es/ecma262/) to resolve behavior, especially coercion, property order, IteratorClose, completion records, and promise jobs.

### Recommended first implementation sequence

1. M0 failure reporting, timeouts, source-goal handling, and compiler diagnostics.
2. Shared primitive/error semantics: `truthiness-coercion.js`, `equality-edge-cases.js`, `error-types.js`, and `nullish-property-access.js`.
3. True lazy evaluation: `logical-shortcircuit-side-effects.js`, `nullish-shortcircuit.js`, `optional-chain-side-effects.js`, and `optional-call.js`.
4. Lexical binding enforcement and missing operators, followed by completion/finally handling and the UTF-16 foundation.
5. M2's function/property/iterator operations, then small M3 library increments and M4 promise settlement/chaining.

This sequence turns the existing examples into a stronger semantic base while creating reusable implementation paths for the new features.
