# go_js failure corpus

**135 standalone JavaScript programs that fail against the `go_js` MCP tool.**

Generated 2026-11-12 by running each snippet verbatim through `tools.go_js.execute({ code })`.

## Layout

- `NNN-<category>-<name>.js` — one failing snippet per file, exactly as executed.
- `results.json` — machine-readable results (`id`, `file`, `category`, `severity`, `kind`, `expected`, `actual`/`detail`, `code`).
- `manifest.tsv` — flat index.

Files are byte-for-byte the source passed to the tool. The EOF-sensitive repros deliberately have **no trailing newline** — if an editor adds one on save, the `.x`/`.`/`\` parser panics stop reproducing.

## How to reproduce

Read a file **without appending a newline** and pass its contents verbatim:

```js
await tools.go_js.execute({ code });
```

Or paste a file's contents into the go_js editor and execute it. Every call gets a fresh process.

## Result mix

| Kind | Count | Meaning |
| --- | ---: | --- |
| `runtime-error` | 57 | Program threw |
| `output-mismatch` | 50 | Program ran but stdout differed from the standard result |
| `panic` | 14 | Go/VM panic (process exit 2) |
| `parse-error` | 9 | Tool rejected the program with a syntax error |
| `accepted-invalid` | 4 | An invalid program was accepted and ran |
| `compile-error` | 1 | Module/compile stage failed instead of parsing as a script |

Severity mix: **53** low, **40** high, **28** medium, **14** critical.

## Headline findings

1. **The parser panics on valid input.** Any program ending in `.x` (a single-character property with no trailing whitespace) crashes Go in `readToken_dot` with `index out of range`. `var o={x:1}; o.x` is enough. Lone `\`, lone `.`, trailing `a.`, `a..b`, an unterminated template, and `1e` also panic; `return\n5` panics in the compiler.
2. **The VM panics on ordinary patterns.** A generator used as `Symbol.iterator` panics in `vm.currentFrame`; `setTimeout(fn)` with no delay panics in `vm.pop`; `class A extends Array` followed by `a.push(1)` panics in `vm.run`.
3. **`with` is a no-op.** Its body is skipped entirely: no output, no assignment, no `var` hoisting.
4. **Sloppy-mode `this` is `undefined`.** Function calls and top-level `this` behave as if always strict; arrow functions don't capture a global `this`.
5. **Optional calls on primitives throw.** `1?.toString()` and `'x'?.toUpperCase()` fail with `Value is not callable`, while `obj.method?.()` works.
6. **Static blocks can't reference their class.** `static { A.x = 5 }` throws a TDZ `ReferenceError`; `static { this.x = 5 }` works.
7. **`JSON.stringify` rejects host objects.** `Date`, `Map`, `Set`, boxed primitives, `RegExp`, `Error`, and typed arrays all serialize to `undefined`; nested ones are dropped or become `null`.
8. **`instanceof` is broken for core types.** `[] instanceof Array`, `fn instanceof Function`, `new Date() instanceof Date`, `new Map() instanceof Map`, `new Set() instanceof Set`, and `/x/ instanceof RegExp` all throw `function has non-object prototype in instanceof check`. `Map.prototype` is `undefined` and built-in `prototype.constructor` links are missing.
9. **Timers misbehave.** `setTimeout(fn, 0, arg)` rejects the callback, `clearTimeout(id)` cancels *all* pending timers, and `clearInterval(id)` inside its own callback throws `Value is not callable`.
10. **`URL` is a stub.** `username`, `password`, `port`, `hash`, and `origin` are missing/undefined; `search` loses its `?`; `toString()` returns `[object Object]`; the `pathname` setter is ignored; relative resolution is wrong; `URLSearchParams` and `URL.canParse` are absent.

## Category summary

| Category | Files | Severity | What it means |
| --- | ---: | --- | --- |
| `panic-parser` | 9 | critical | The Go parser panics and exits with status 2 instead of returning a JS error. |
| `panic-vm` | 5 | critical | The VM/compiler panics instead of throwing a JS exception. |
| `parser-invalid` | 6 | high | Invalid programs are accepted, executed, or resolved as files instead of being rejected. |
| `syntax-support` | 9 | medium | Valid (or web-legacy) JavaScript is rejected by the parser. |
| `semantics` | 12 | high | The program runs but produces the wrong result. |
| `instanceof-prototype` | 10 | high | Built-in constructor/prototype plumbing is damaged; `instanceof` throws for several types. |
| `json` | 11 | medium | `JSON.stringify` drops or mangles non-plain objects. |
| `runtime-apis` | 5 | medium | A standard runtime API is missing or rejects valid input. |
| `timers` | 4 | high | Timer scheduling/clearing behaves incorrectly. |
| `console` | 10 | low | Console methods are missing or formatting is wrong. |
| `modern-missing` | 17 | low | A newer standard API is absent. |
| `weak-collections` | 2 | medium | `WeakMap`/`WeakSet` exist as constructors but their instances have no methods. |
| `host-missing` | 19 | low | A browser/Node host API is absent. May be out of scope for this sandbox. |
| `url` | 9 | high | The WHATWG `URL` implementation is a stub with wrong fields and parsing. |
| `intl` | 7 | low | `Intl` only implements `NumberFormat`; the rest are absent. |

## Notes and exclusions

- Invalid programs that produce a **clean** `SyntaxError` (e.g. `0x`, `0b2`, an unterminated string) are working as intended and aren't listed.
- Uncaught exceptions that are supposed to terminate the job (`throw`, a throw inside a timer) aren't listed.
- A NUL byte in the source can't even be queued: the backend's UTF-8 column rejects `0x00` (`SQLSTATE 22021`), so that case is infrastructure, not JS.
- Parallel calls fail with `the editor is already running a request`; the tool serialises jobs. This is an operational limitation, not a snippet.
- Unhandled promise rejections are silent (`Promise.reject(new Error('x'))` exits 0 with no output). There is no defined stdout expectation for that, so it isn't a snippet file.
- `import x from 'y'` doesn't produce a `SyntaxError`; it enters module compilation and tries to open `y` as a file (`compile error: open y: ...`). Module specifiers therefore reach the filesystem resolver.
- Missing browser/Node host APIs (`fetch`, `TextEncoder`, `process`, ...) are tagged `host-missing` / `low`; they may be intentionally out of scope. Missing recent ES APIs are tagged `modern-missing` / `low`.
- `console.log({a:1})` emits an extra newline and `%s` isn't substituted. These are tagged `console` / `low` because console formatting isn't standardised.

## Failures

### Parser panic (`panic-parser`) — 9

The Go parser panics and exits with status 2 instead of returning a JS error.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 001 | [001-panic-parser-dot-eof-null.js](001-panic-parser-dot-eof-null.js) | `panic` | `"(runtime TypeError)"` | panic: runtime error: index out of range [6] with length 6 @ go_js/parser.(*Parser).readToken_dot(0x…) |
| 002 | [002-panic-parser-dot-eof-absolute.js](002-panic-parser-dot-eof-absolute.js) | `panic` | `""` | panic: runtime error: index out of range [16] with length 16 @ go_js/parser.(*Parser).readToken_dot(0x…) |
| 003 | [003-panic-parser-trailing-dot.js](003-panic-parser-trailing-dot.js) | `panic` | `"(clean SyntaxError)"` | panic: runtime error: index out of range [12] with length 12 @ go_js/parser.(*Parser).readToken_dot(0x…) |
| 004 | [004-panic-parser-double-dot.js](004-panic-parser-double-dot.js) | `panic` | `"(clean SyntaxError)"` | panic: runtime error: index out of range [21] with length 21 @ go_js/parser.(*Parser).readToken_dot(0x…) |
| 005 | [005-panic-parser-lone-dot.js](005-panic-parser-lone-dot.js) | `panic` | `"(clean SyntaxError)"` | panic: runtime error: index out of range [1] with length 1 @ go_js/parser.(*Parser).readToken_dot(0x…) |
| 006 | [006-panic-parser-lone-backslash.js](006-panic-parser-lone-backslash.js) | `panic` | `"(clean SyntaxError)"` | panic: runtime error: index out of range [1] with length 1 @ go_js/parser.(*Parser).readWord1(0x…) |
| 007 | [007-panic-parser-unterminated-template.js](007-panic-parser-unterminated-template.js) | `panic` | `"(clean SyntaxError)"` | panic: interface conversion: interface {} is nil, not []uint8 @ go_js/parser.(*Parser).parseTemplateElement(0x…, {0x…}) |
| 008 | [008-panic-parser-bad-exponent.js](008-panic-parser-bad-exponent.js) | `panic` | `"(clean SyntaxError)"` | panic: runtime error: index out of range [2] with length 2 @ go_js/parser.(*Parser).readNumber(0x…, 0x…) |
| 009 | [009-panic-parser-return-newline.js](009-panic-parser-return-newline.js) | `panic` | `"undefined\n"` | panic: runtime error: invalid memory address or nil pointer dereference @ go_js/compiler.argumentIsPromise(0x…) |

### VM panic (`panic-vm`) — 5

The VM/compiler panics instead of throwing a JS exception.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 010 | [010-panic-vm-generator-iterable-spread.js](010-panic-vm-generator-iterable-spread.js) | `panic` | `"1,2\n"` | panic: runtime error: index out of range [-1] @ go_js/vm.(*VM).currentFrame(...) |
| 011 | [011-panic-vm-generator-method-symbol.js](011-panic-vm-generator-method-symbol.js) | `panic` | `"1\n"` | panic: runtime error: index out of range [-1] @ go_js/vm.(*VM).currentFrame(...) |
| 012 | [012-panic-vm-array-from-generator.js](012-panic-vm-array-from-generator.js) | `panic` | `"1\n"` | panic: runtime error: index out of range [-1] @ go_js/vm.(*VM).currentFrame(...) |
| 013 | [013-panic-vm-settimeout-no-delay.js](013-panic-vm-settimeout-no-delay.js) | `panic` | `"x\n"` | panic: runtime error: index out of range [-1] @ go_js/vm.(*VM).pop(...) |
| 014 | [014-panic-vm-extends-array-push.js](014-panic-vm-extends-array-push.js) | `panic` | `""` | panic: interface conversion: object.Object is *native.Instance, not *native.ObjArr @ go_js/vm.(*VM).run(0x…, {{0x…, 0x…}, 0x…, 0x…, 0x…, 0x…, 0x…, 0x…, 0x…, ...}, ...) |

### Invalid input mishandled (`parser-invalid`) — 6

Invalid programs are accepted, executed, or resolved as files instead of being rejected.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 015 | [015-parser-invalid-unterminated-comment.js](015-parser-invalid-unterminated-comment.js) | `runtime-error` | `"(clean SyntaxError)"` | ReferenceError: abc is not defined |
| 016 | [016-parser-invalid-bad-unicode-escape.js](016-parser-invalid-bad-unicode-escape.js) | `accepted-invalid` | — | program was accepted; output="\u0000{}\n" |
| 017 | [017-parser-invalid-strict-with.js](017-parser-invalid-strict-with.js) | `accepted-invalid` | — | program was accepted; output="" |
| 018 | [018-parser-invalid-strict-duplicate-params.js](018-parser-invalid-strict-duplicate-params.js) | `accepted-invalid` | — | program was accepted; output="" |
| 019 | [019-parser-invalid-export-declaration.js](019-parser-invalid-export-declaration.js) | `accepted-invalid` | — | program was accepted; output="" |
| 020 | [020-parser-invalid-import-declaration.js](020-parser-invalid-import-declaration.js) | `compile-error` | `"(clean SyntaxError)"` | Error: open y: no such file or directory |

### Valid syntax rejected (`syntax-support`) — 9

Valid (or web-legacy) JavaScript is rejected by the parser.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 021 | [021-syntax-support-html-open-comment.js](021-syntax-support-html-open-comment.js) | `parse-error` | `"1\n"` | SyntaxError: Unexpected token: parseExprAtomDefault() (1:0) |
| 022 | [022-syntax-support-html-close-comment.js](022-syntax-support-html-close-comment.js) | `parse-error` | `"1\n"` | SyntaxError: Unexpected token: parseExprAtomDefault() (2:2) |
| 023 | [023-syntax-support-yield-identifier.js](023-syntax-support-yield-identifier.js) | `parse-error` | `"1\n"` | SyntaxError: The keyword yield is reserved (1:4) |
| 024 | [024-syntax-support-await-identifier.js](024-syntax-support-await-identifier.js) | `parse-error` | `"1\n"` | SyntaxError: Cannot use keyword 'await' outside an async function (1:4) |
| 025 | [025-syntax-support-legacy-octal-literal.js](025-syntax-support-legacy-octal-literal.js) | `parse-error` | `"8\n"` | SyntaxError: Unexpected token: hanging comma (1:15) |
| 026 | [026-syntax-support-legacy-octal-escape.js](026-syntax-support-legacy-octal-escape.js) | `output-mismatch` | `"A\n"` | `"\n"` |
| 027 | [027-syntax-support-unicode-ident-cafe.js](027-syntax-support-unicode-ident-cafe.js) | `parse-error` | `"3\n"` | SyntaxError: Unexpected token: Missing initializer in for..of loop (1:9) |
| 028 | [028-syntax-support-unicode-ident-greek.js](028-syntax-support-unicode-ident-greek.js) | `parse-error` | `"3\n"` | SyntaxError: Unexpected keyword var (1:4) |
| 029 | [029-syntax-support-unicode-ident-cjk.js](029-syntax-support-unicode-ident-cjk.js) | `parse-error` | `"4\n"` | SyntaxError: Unexpected keyword var (1:4) |

### Wrong semantics (`semantics`) — 12

The program runs but produces the wrong result.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 030 | [030-semantics-with-body-skipped.js](030-semantics-with-body-skipped.js) | `output-mismatch` | `"hit\n"` | `""` |
| 031 | [031-semantics-with-assignment-ignored.js](031-semantics-with-assignment-ignored.js) | `output-mismatch` | `"2\n"` | `"1\n"` |
| 032 | [032-semantics-with-var-not-hoisted.js](032-semantics-with-var-not-hoisted.js) | `output-mismatch` | `"number\n"` | `"undefined\n"` |
| 033 | [033-semantics-sloppy-this-type.js](033-semantics-sloppy-this-type.js) | `output-mismatch` | `"object\n"` | `"undefined\n"` |
| 034 | [034-semantics-top-level-this.js](034-semantics-top-level-this.js) | `output-mismatch` | `"object\n"` | `"undefined\n"` |
| 035 | [035-semantics-arrow-this.js](035-semantics-arrow-this.js) | `output-mismatch` | `"true\n"` | `"false\n"` |
| 036 | [036-semantics-optional-call-number-literal.js](036-semantics-optional-call-number-literal.js) | `runtime-error` | `"1\n"` | TypeError: Value is not callable |
| 037 | [037-semantics-optional-call-number-var.js](037-semantics-optional-call-number-var.js) | `runtime-error` | `"1\n"` | TypeError: Value is not callable |
| 038 | [038-semantics-optional-call-string.js](038-semantics-optional-call-string.js) | `runtime-error` | `"X\n"` | TypeError: Value is not callable |
| 039 | [039-semantics-static-block-class-ref.js](039-semantics-static-block-class-ref.js) | `runtime-error` | `"5\n"` | ReferenceError: Cannot access variable before initialization |
| 040 | [040-semantics-static-block-class-assign.js](040-semantics-static-block-class-assign.js) | `runtime-error` | `"5\n"` | ReferenceError: Cannot access variable before initialization |
| 041 | [041-semantics-sloppy-this-global.js](041-semantics-sloppy-this-global.js) | `output-mismatch` | `"true\n"` | `"false\n"` |

### Prototype / instanceof damage (`instanceof-prototype`) — 10

Built-in constructor/prototype plumbing is damaged; `instanceof` throws for several types.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 042 | [042-instanceof-prototype-instanceof-array.js](042-instanceof-prototype-instanceof-array.js) | `runtime-error` | `"true\n"` | TypeError: function has non-object prototype in instanceof check |
| 043 | [043-instanceof-prototype-instanceof-function.js](043-instanceof-prototype-instanceof-function.js) | `runtime-error` | `"true\n"` | TypeError: function has non-object prototype in instanceof check |
| 044 | [044-instanceof-prototype-instanceof-date.js](044-instanceof-prototype-instanceof-date.js) | `runtime-error` | `"true\n"` | TypeError: function has non-object prototype in instanceof check |
| 045 | [045-instanceof-prototype-instanceof-map.js](045-instanceof-prototype-instanceof-map.js) | `runtime-error` | `"true\n"` | TypeError: function has non-object prototype in instanceof check |
| 046 | [046-instanceof-prototype-instanceof-set.js](046-instanceof-prototype-instanceof-set.js) | `runtime-error` | `"true\n"` | TypeError: function has non-object prototype in instanceof check |
| 047 | [047-instanceof-prototype-instanceof-regexp.js](047-instanceof-prototype-instanceof-regexp.js) | `runtime-error` | `"true\n"` | TypeError: function has non-object prototype in instanceof check |
| 048 | [048-instanceof-prototype-map-prototype-missing.js](048-instanceof-prototype-map-prototype-missing.js) | `output-mismatch` | `"object,true\n"` | `"undefined,false\n"` |
| 049 | [049-instanceof-prototype-array-proto-constructor.js](049-instanceof-prototype-array-proto-constructor.js) | `output-mismatch` | `"true\n"` | `"false\n"` |
| 050 | [050-instanceof-prototype-error-proto-constructor.js](050-instanceof-prototype-error-proto-constructor.js) | `output-mismatch` | `"true\n"` | `"false\n"` |
| 051 | [051-instanceof-prototype-extends-map-method.js](051-instanceof-prototype-extends-map-method.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |

### JSON serialization (`json`) — 11

`JSON.stringify` drops or mangles non-plain objects.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 052 | [052-json-json-date.js](052-json-json-date.js) | `output-mismatch` | `"\"1970-01-01T00:00:00.000Z\"\n"` | `"undefined\n"` |
| 053 | [053-json-json-map.js](053-json-json-map.js) | `output-mismatch` | `"{}\n"` | `"undefined\n"` |
| 054 | [054-json-json-set.js](054-json-json-set.js) | `output-mismatch` | `"{}\n"` | `"undefined\n"` |
| 055 | [055-json-json-number-object.js](055-json-json-number-object.js) | `output-mismatch` | `"5\n"` | `"undefined\n"` |
| 056 | [056-json-json-string-object.js](056-json-json-string-object.js) | `output-mismatch` | `"\"x\"\n"` | `"undefined\n"` |
| 057 | [057-json-json-bool-object.js](057-json-json-bool-object.js) | `output-mismatch` | `"true\n"` | `"undefined\n"` |
| 058 | [058-json-json-regexp.js](058-json-json-regexp.js) | `output-mismatch` | `"{}\n"` | `"undefined\n"` |
| 059 | [059-json-json-error.js](059-json-json-error.js) | `output-mismatch` | `"{}\n"` | `"undefined\n"` |
| 060 | [060-json-json-typedarray.js](060-json-json-typedarray.js) | `output-mismatch` | `"{\"0\":1,\"1\":2}\n"` | `"undefined\n"` |
| 061 | [061-json-json-nested-host.js](061-json-json-nested-host.js) | `output-mismatch` | `"{\"m\":{},\"d\":\"1970-01-01T00:00:00.000Z\"}\n"` | `"{}\n"` |
| 062 | [062-json-json-array-host.js](062-json-json-array-host.js) | `output-mismatch` | `"[{},\"1970-01-01T00:00:00.000Z\"]\n"` | `"[null,null]\n"` |

### Runtime API (Reflect / generators / eval) (`runtime-apis`) — 5

A standard runtime API is missing or rejects valid input.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 063 | [063-runtime-apis-reflect-apply-parseint.js](063-runtime-apis-reflect-apply-parseint.js) | `runtime-error` | `"42\n"` | TypeError: Reflect.apply target is not callable |
| 064 | [064-runtime-apis-reflect-apply-mathmax.js](064-runtime-apis-reflect-apply-mathmax.js) | `runtime-error` | `"9\n"` | TypeError: Reflect.apply target is not callable |
| 065 | [065-runtime-apis-generator-return-method.js](065-runtime-apis-generator-return-method.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |
| 066 | [066-runtime-apis-generator-throw-method.js](066-runtime-apis-generator-throw-method.js) | `runtime-error` | `"caught\n"` | TypeError: Value is not callable |
| 067 | [067-runtime-apis-eval-missing.js](067-runtime-apis-eval-missing.js) | `runtime-error` | `"2\n"` | ReferenceError: eval is not defined |

### Timers (`timers`) — 4

Timer scheduling/clearing behaves incorrectly.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 068 | [068-timers-settimeout-extra-args.js](068-timers-settimeout-extra-args.js) | `runtime-error` | `"x\n"` | TypeError: The callback argument must be of type function |
| 069 | [069-timers-cleartimeout-cancels-other.js](069-timers-cleartimeout-cancels-other.js) | `output-mismatch` | `"b\n"` | `""` |
| 070 | [070-timers-cleartimeout-cancels-all.js](070-timers-cleartimeout-cancels-all.js) | `output-mismatch` | `"a\n"` | `""` |
| 071 | [071-timers-clearinterval-in-callback.js](071-timers-clearinterval-in-callback.js) | `runtime-error` | `"cleared\n"` | TypeError: Value is not callable |

### Console API (`console`) — 10

Console methods are missing or formatting is wrong.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 072 | [072-console-console-warn.js](072-console-console-warn.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 073 | [073-console-console-info.js](073-console-console-info.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 074 | [074-console-console-debug.js](074-console-console-debug.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 075 | [075-console-console-assert.js](075-console-console-assert.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 076 | [076-console-console-dir.js](076-console-console-dir.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 077 | [077-console-console-trace.js](077-console-console-trace.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 078 | [078-console-console-count.js](078-console-console-count.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 079 | [079-console-console-group.js](079-console-console-group.js) | `runtime-error` | `"(no throw)"` | TypeError: Value is not callable |
| 080 | [080-console-console-format-s.js](080-console-console-format-s.js) | `output-mismatch` | `"x\n"` | `"%s x\n"` |
| 081 | [081-console-console-object-extra-newline.js](081-console-console-object-extra-newline.js) | `output-mismatch` | `"{ a: 1 }\n"` | `"{ a: 1 }\n\n"` |

### Missing modern ES feature (`modern-missing`) — 17

A newer standard API is absent.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 082 | [082-modern-missing-promise-withresolvers.js](082-modern-missing-promise-withresolvers.js) | `runtime-error` | `"5\n"` | TypeError: Value is not callable |
| 083 | [083-modern-missing-array-fromasync.js](083-modern-missing-array-fromasync.js) | `runtime-error` | `"1,2\n"` | TypeError: Value is not callable |
| 084 | [084-modern-missing-string-iswellformed.js](084-modern-missing-string-iswellformed.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |
| 085 | [085-modern-missing-string-towellformed.js](085-modern-missing-string-towellformed.js) | `runtime-error` | `"1\n"` | TypeError: Value is not callable |
| 086 | [086-modern-missing-arraybuffer-transfer.js](086-modern-missing-arraybuffer-transfer.js) | `runtime-error` | `"4,0\n"` | TypeError: Value is not callable |
| 087 | [087-modern-missing-regexp-escape.js](087-modern-missing-regexp-escape.js) | `runtime-error` | `"a\\.b\n"` | TypeError: Value is not callable |
| 088 | [088-modern-missing-iterator-from.js](088-modern-missing-iterator-from.js) | `runtime-error` | `"1,2\n"` | ReferenceError: Iterator is not defined |
| 089 | [089-modern-missing-promise-try.js](089-modern-missing-promise-try.js) | `runtime-error` | `"7\n"` | TypeError: Value is not callable |
| 090 | [090-modern-missing-error-iserror.js](090-modern-missing-error-iserror.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |
| 091 | [091-modern-missing-set-union.js](091-modern-missing-set-union.js) | `runtime-error` | `"1,2,3\n"` | TypeError: Value is not callable |
| 092 | [092-modern-missing-set-issubsetof.js](092-modern-missing-set-issubsetof.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |
| 093 | [093-modern-missing-map-groupby.js](093-modern-missing-map-groupby.js) | `runtime-error` | `"1,3\n"` | TypeError: Value is not callable |
| 094 | [094-modern-missing-u8-tobase64.js](094-modern-missing-u8-tobase64.js) | `runtime-error` | `"aGk=\n"` | TypeError: Value is not callable |
| 095 | [095-modern-missing-explicit-resource-management.js](095-modern-missing-explicit-resource-management.js) | `parse-error` | `"disposed\n"` | SyntaxError: Unexpected token: Unexpected token (1:7) |
| 096 | [096-modern-missing-float16array-type.js](096-modern-missing-float16array-type.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 097 | [097-modern-missing-symbol-dispose.js](097-modern-missing-symbol-dispose.js) | `output-mismatch` | `"symbol\n"` | `"undefined\n"` |
| 098 | [098-modern-missing-math-sumprecise.js](098-modern-missing-math-sumprecise.js) | `runtime-error` | `"0.30000000000000004\n"` | TypeError: Value is not callable |

### WeakMap / WeakSet (`weak-collections`) — 2

`WeakMap`/`WeakSet` exist as constructors but their instances have no methods.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 099 | [099-weak-collections-weakmap-methods.js](099-weak-collections-weakmap-methods.js) | `output-mismatch` | `"function,function,function,function\n"` | `"undefined,undefined,undefined,undefined\n"` |
| 100 | [100-weak-collections-weakset-methods.js](100-weak-collections-weakset-methods.js) | `output-mismatch` | `"function,function,function\n"` | `"undefined,undefined,undefined\n"` |

### Missing host API (`host-missing`) — 19

A browser/Node host API is absent. May be out of scope for this sandbox.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 101 | [101-host-missing-textencoder.js](101-host-missing-textencoder.js) | `runtime-error` | `"1\n"` | ReferenceError: TextEncoder is not defined |
| 102 | [102-host-missing-textdecoder.js](102-host-missing-textdecoder.js) | `runtime-error` | `"a\n"` | ReferenceError: TextDecoder is not defined |
| 103 | [103-host-missing-atob.js](103-host-missing-atob.js) | `runtime-error` | `"hi\n"` | ReferenceError: atob is not defined |
| 104 | [104-host-missing-btoa.js](104-host-missing-btoa.js) | `runtime-error` | `"aGk=\n"` | ReferenceError: btoa is not defined |
| 105 | [105-host-missing-structuredclone.js](105-host-missing-structuredclone.js) | `runtime-error` | `"1\n"` | ReferenceError: structuredClone is not defined |
| 106 | [106-host-missing-crypto.js](106-host-missing-crypto.js) | `runtime-error` | `"4\n"` | ReferenceError: crypto is not defined |
| 107 | [107-host-missing-performance.js](107-host-missing-performance.js) | `runtime-error` | `"number\n"` | ReferenceError: performance is not defined |
| 108 | [108-host-missing-fetch.js](108-host-missing-fetch.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 109 | [109-host-missing-requestanimationframe.js](109-host-missing-requestanimationframe.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 110 | [110-host-missing-node-process.js](110-host-missing-node-process.js) | `output-mismatch` | `"object\n"` | `"undefined\n"` |
| 111 | [111-host-missing-node-require.js](111-host-missing-node-require.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 112 | [112-host-missing-node-buffer.js](112-host-missing-node-buffer.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 113 | [113-host-missing-setimmediate.js](113-host-missing-setimmediate.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 114 | [114-host-missing-weakref.js](114-host-missing-weakref.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 115 | [115-host-missing-finalizationregistry.js](115-host-missing-finalizationregistry.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 116 | [116-host-missing-sharedarraybuffer.js](116-host-missing-sharedarraybuffer.js) | `output-mismatch` | `"function\n"` | `"undefined\n"` |
| 117 | [117-host-missing-atomics.js](117-host-missing-atomics.js) | `output-mismatch` | `"object\n"` | `"undefined\n"` |
| 118 | [118-host-missing-webassembly.js](118-host-missing-webassembly.js) | `output-mismatch` | `"object\n"` | `"undefined\n"` |
| 119 | [119-host-missing-temporal.js](119-host-missing-temporal.js) | `output-mismatch` | `"object\n"` | `"undefined\n"` |

### URL (`url`) — 9

The WHATWG `URL` implementation is a stub with wrong fields and parsing.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 120 | [120-url-url-components.js](120-url-url-components.js) | `output-mismatch` | `"https:\|user\|pass\|e.com\|8080\|/a\|?b=1\|#c\|https://e.com:8080\n"` | `"https:\|\|\|e.com\|\|/a\|b=1\|\|\n"` |
| 121 | [121-url-url-tostring.js](121-url-url-tostring.js) | `output-mismatch` | `"https://e.com/a\n"` | `"[object Object]\n"` |
| 122 | [122-url-url-pathname-set.js](122-url-url-pathname-set.js) | `output-mismatch` | `"https://e.com/b\n"` | `"https://e.com/a\n"` |
| 123 | [123-url-url-relative.js](123-url-url-relative.js) | `output-mismatch` | `"https://e.com/x\n"` | `"/x\n"` |
| 124 | [124-url-url-search.js](124-url-url-search.js) | `output-mismatch` | `"?x=1\n"` | `"x=1\n"` |
| 125 | [125-url-url-hash.js](125-url-url-hash.js) | `output-mismatch` | `"#h\n"` | `"undefined\n"` |
| 126 | [126-url-url-port.js](126-url-url-port.js) | `output-mismatch` | `"8080\n"` | `"undefined\n"` |
| 127 | [127-url-urlsearchparams-missing.js](127-url-urlsearchparams-missing.js) | `runtime-error` | `"2\n"` | ReferenceError: URLSearchParams is not defined |
| 128 | [128-url-url-canparse.js](128-url-url-canparse.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |

### Intl (`intl`) — 7

`Intl` only implements `NumberFormat`; the rest are absent.

| # | File | Kind | Expected | Actual / error |
| ---: | --- | --- | --- | --- |
| 129 | [129-intl-intl-datetimeformat.js](129-intl-intl-datetimeformat.js) | `runtime-error` | `"1/1/1970\n"` | TypeError: Value is not callable |
| 130 | [130-intl-intl-collator.js](130-intl-intl-collator.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |
| 131 | [131-intl-intl-listformat.js](131-intl-intl-listformat.js) | `runtime-error` | `"a and b\n"` | TypeError: Value is not callable |
| 132 | [132-intl-intl-pluralrules.js](132-intl-intl-pluralrules.js) | `runtime-error` | `"one\n"` | TypeError: Value is not callable |
| 133 | [133-intl-intl-relativetimeformat.js](133-intl-intl-relativetimeformat.js) | `runtime-error` | `"1 day ago\n"` | TypeError: Value is not callable |
| 134 | [134-intl-intl-displaynames.js](134-intl-intl-displaynames.js) | `runtime-error` | `"French\n"` | TypeError: Value is not callable |
| 135 | [135-intl-intl-segmenter.js](135-intl-intl-segmenter.js) | `runtime-error` | `"true\n"` | TypeError: Value is not callable |

## Root-cause quick reference

- **dot-at-EOF panic**: [002-panic-parser-dot-eof-absolute.js](002-panic-parser-dot-eof-absolute.js), [001-panic-parser-dot-eof-null.js](001-panic-parser-dot-eof-null.js), [003-panic-parser-trailing-dot.js](003-panic-parser-trailing-dot.js), [004-panic-parser-double-dot.js](004-panic-parser-double-dot.js), [005-panic-parser-lone-dot.js](005-panic-parser-lone-dot.js)
- **other parser panics**: [006-panic-parser-lone-backslash.js](006-panic-parser-lone-backslash.js), [007-panic-parser-unterminated-template.js](007-panic-parser-unterminated-template.js), [008-panic-parser-bad-exponent.js](008-panic-parser-bad-exponent.js), [009-panic-parser-return-newline.js](009-panic-parser-return-newline.js)
- **generator-as-iterator panic**: [010-panic-vm-generator-iterable-spread.js](010-panic-vm-generator-iterable-spread.js), [011-panic-vm-generator-method-symbol.js](011-panic-vm-generator-method-symbol.js), [012-panic-vm-array-from-generator.js](012-panic-vm-array-from-generator.js)
- **timer panics / corruption**: [013-panic-vm-settimeout-no-delay.js](013-panic-vm-settimeout-no-delay.js), [068-timers-settimeout-extra-args.js](068-timers-settimeout-extra-args.js), [069-timers-cleartimeout-cancels-other.js](069-timers-cleartimeout-cancels-other.js), [070-timers-cleartimeout-cancels-all.js](070-timers-cleartimeout-cancels-all.js), [071-timers-clearinterval-in-callback.js](071-timers-clearinterval-in-callback.js)
- **Array subclass panic**: [014-panic-vm-extends-array-push.js](014-panic-vm-extends-array-push.js)
- **`with` no-op**: [030-semantics-with-body-skipped.js](030-semantics-with-body-skipped.js), [031-semantics-with-assignment-ignored.js](031-semantics-with-assignment-ignored.js), [032-semantics-with-var-not-hoisted.js](032-semantics-with-var-not-hoisted.js)
- **sloppy `this`**: [033-semantics-sloppy-this-type.js](033-semantics-sloppy-this-type.js), [041-semantics-sloppy-this-global.js](041-semantics-sloppy-this-global.js), [034-semantics-top-level-this.js](034-semantics-top-level-this.js), [035-semantics-arrow-this.js](035-semantics-arrow-this.js)
- **optional call on primitives**: [036-semantics-optional-call-number-literal.js](036-semantics-optional-call-number-literal.js), [037-semantics-optional-call-number-var.js](037-semantics-optional-call-number-var.js), [038-semantics-optional-call-string.js](038-semantics-optional-call-string.js)
- **static block TDZ**: [039-semantics-static-block-class-ref.js](039-semantics-static-block-class-ref.js), [040-semantics-static-block-class-assign.js](040-semantics-static-block-class-assign.js)
- **prototype / instanceof**: [042-instanceof-prototype-instanceof-array.js](042-instanceof-prototype-instanceof-array.js), [043-instanceof-prototype-instanceof-function.js](043-instanceof-prototype-instanceof-function.js), [045-instanceof-prototype-instanceof-map.js](045-instanceof-prototype-instanceof-map.js), [048-instanceof-prototype-map-prototype-missing.js](048-instanceof-prototype-map-prototype-missing.js), [049-instanceof-prototype-array-proto-constructor.js](049-instanceof-prototype-array-proto-constructor.js), [051-instanceof-prototype-extends-map-method.js](051-instanceof-prototype-extends-map-method.js)
- **JSON host objects**: [052-json-json-date.js](052-json-json-date.js), [053-json-json-map.js](053-json-json-map.js), [055-json-json-number-object.js](055-json-json-number-object.js), [060-json-json-typedarray.js](060-json-json-typedarray.js), [061-json-json-nested-host.js](061-json-json-nested-host.js), [062-json-json-array-host.js](062-json-json-array-host.js)
- **URL stub**: [120-url-url-components.js](120-url-url-components.js), [121-url-url-tostring.js](121-url-url-tostring.js), [122-url-url-pathname-set.js](122-url-url-pathname-set.js), [123-url-url-relative.js](123-url-url-relative.js), [127-url-urlsearchparams-missing.js](127-url-urlsearchparams-missing.js), [128-url-url-canparse.js](128-url-url-canparse.js)
- **missing console methods**: [072-console-console-warn.js](072-console-console-warn.js), [073-console-console-info.js](073-console-console-info.js), [074-console-console-debug.js](074-console-console-debug.js), [075-console-console-assert.js](075-console-console-assert.js), [077-console-console-trace.js](077-console-console-trace.js)
- **missing Intl**: [129-intl-intl-datetimeformat.js](129-intl-intl-datetimeformat.js), [130-intl-intl-collator.js](130-intl-intl-collator.js), [131-intl-intl-listformat.js](131-intl-intl-listformat.js), [135-intl-intl-segmenter.js](135-intl-intl-segmenter.js)
- **weak collections**: [099-weak-collections-weakmap-methods.js](099-weak-collections-weakmap-methods.js), [100-weak-collections-weakset-methods.js](100-weak-collections-weakset-methods.js)

