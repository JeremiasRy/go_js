// Probe 04 — an empty `for(;;)` crashes the go_js compiler.
//
// Node 24 baseline: prints "for(;;) -> 1" and exits 0.
//
// go_js observed (2026-11-14): hard VM panic before any JS runs:
//
//   panic: runtime error: invalid memory address or nil pointer dereference
//   [signal SIGSEGV ...]
//   go_js/compiler.withAstTracking(...)
//   go_js/compiler.generateByteCode(...)
//   ...
//   main.main()  /app/main.go:116
//
// Workaround: write `while (true)` instead (verified to compile and run).

function f() {
  for (;;) {
    break;
  }
  return 1;
}

console.log('for(;;) ->', f());
